# ======== niffy.mk =========
# Niffy release packaging and local verification.

NIFFY_VERSION := $(shell sed -n 's/^version:[[:space:]]*//p' config/niffy/metadata.yaml | head -n1)
NIFFY_TAG ?= v$(NIFFY_VERSION)
NIFFY_REGISTRY ?= niffy
NIFFY_ROUTER_IMAGE ?= $(NIFFY_REGISTRY)/router:$(NIFFY_TAG)
NIFFY_DASHBOARD_IMAGE ?= $(NIFFY_REGISTRY)/dashboard:$(NIFFY_TAG)
NIFFY_ENVOY_IMAGE ?= envoyproxy/envoy@sha256:cfc0678bc03cca19cbb031688acb31d510bff501ff97e163026a375fe0515d69
NIFFY_CONFIG ?= config/niffy/config.yaml
NIFFY_MODELS_DIR ?= config/niffy/.vllm-sr/models
NIFFY_MANIFEST ?= config/niffy/.vllm-sr/release-manifest.json
NIFFY_MINIMAL ?= 1
NIFFY_ALLOW_DIRTY ?= 0
NIFFY_CODE_LEDGER ?=
NIFFY_CODE_REPORT ?= $(CURDIR)/.agent-harness/niffy/code-evaluation.json

.PHONY: niffy-release niffy-release-build niffy-release-manifest \
	niffy-release-verify niffy-release-start niffy-release-stop \
	niffy-code-eval niffy-code-eval-test

niffy-code-eval: ## Gate a coding evaluation ledger on validation, cost, and escalation evidence
	@test -n "$(NIFFY_CODE_LEDGER)" || { echo "NIFFY_CODE_LEDGER is required"; exit 2; }
	@$(AGENT_PYTHON) tools/release/niffy_code_evaluation.py \
		"$(NIFFY_CODE_LEDGER)" --output "$(NIFFY_CODE_REPORT)"

niffy-code-eval-test: ## Test the NIFFY-12 coding outcome gate
	@PYTHONPATH=tools/release $(AGENT_PYTHON) -m unittest \
		tools/release/test_niffy_code_evaluation.py

niffy-release: niffy-release-build niffy-release-manifest ## Build versioned Niffy images and record their immutable identities

niffy-release-build: ## Build the versioned Niffy router/dashboard images through the supported local image flow
	@test -n "$(NIFFY_VERSION)" || { echo "config/niffy/metadata.yaml has no version"; exit 1; }
	@test "$(NIFFY_TAG)" != "latest" || { echo "NIFFY_TAG must be immutable, not latest"; exit 1; }
	@$(MAKE) vllm-sr-router-build \
		VLLM_SR_ROUTER_IMAGE="$(NIFFY_ROUTER_IMAGE)"
	@$(MAKE) vllm-sr-dashboard-build \
		VLLM_SR_DASHBOARD_IMAGE="$(NIFFY_DASHBOARD_IMAGE)" \
		VLLM_SR_DASHBOARD_VERSION="$(NIFFY_VERSION)"
	@$(CONTAINER_RUNTIME) image inspect "$(NIFFY_ENVOY_IMAGE)" >/dev/null 2>&1 || \
		$(CONTAINER_RUNTIME) pull "$(NIFFY_ENVOY_IMAGE)"
	@$(MAKE) vllm-sr-install-cli

niffy-release-manifest: ## Write content-addressed image/config evidence for the Niffy release build
	@$(AGENT_PYTHON) tools/release/niffy_release_manifest.py \
		--version "$(NIFFY_VERSION)" \
		--router-image "$(NIFFY_ROUTER_IMAGE)" \
		--dashboard-image "$(NIFFY_DASHBOARD_IMAGE)" \
		--envoy-image "$(NIFFY_ENVOY_IMAGE)" \
		--config "$(NIFFY_CONFIG)" \
		--models-dir "$(NIFFY_MODELS_DIR)" \
		--metadata config/niffy/metadata.yaml \
		--output "$(NIFFY_MANIFEST)" \
		$(if $(filter 1,$(NIFFY_ALLOW_DIRTY)),--allow-dirty,)

niffy-release-verify: ## Fail if a Niffy image, config, or metadata file drifted after manifest creation
	@$(AGENT_PYTHON) tools/release/niffy_release_manifest.py \
		--check --output "$(NIFFY_MANIFEST)" \
		--router-image "$(NIFFY_ROUTER_IMAGE)" \
		--dashboard-image "$(NIFFY_DASHBOARD_IMAGE)" \
		--envoy-image "$(NIFFY_ENVOY_IMAGE)" \
		--config "$(NIFFY_CONFIG)" \
		--models-dir "$(NIFFY_MODELS_DIR)"

niffy-release-start: niffy-release-verify ## Start only the verified local Niffy release images; never pull a replacement
	@test -f .env || { echo ".env is required"; exit 1; }
	@set -a; . ./.env; set +a; \
		test -n "$$OPENAI_API_KEY" || { echo "OPENAI_API_KEY is required"; exit 1; }; \
		test -n "$$XAI_API_KEY" || { echo "XAI_API_KEY is required"; exit 1; }; \
		test -n "$$MISTRAL_API_KEY" || { echo "MISTRAL_API_KEY is required"; exit 1; }; \
		$(AGENT_VENV)/bin/vllm-sr serve \
			--config "$(NIFFY_CONFIG)" \
			--router-image "$(NIFFY_ROUTER_IMAGE)" \
			--envoy-image "$(NIFFY_ENVOY_IMAGE)" \
			--dashboard-image "$(NIFFY_DASHBOARD_IMAGE)" \
			--image-pull-policy never \
			$(if $(filter 1,$(NIFFY_MINIMAL)),--minimal,)

niffy-release-stop: ## Stop the local Niffy release stack without deleting persistent volumes
	@$(AGENT_VENV)/bin/vllm-sr stop
