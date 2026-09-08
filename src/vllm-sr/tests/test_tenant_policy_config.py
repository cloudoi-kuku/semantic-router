from cli.models import UserConfig
from cli.validator_tenant_policy import validate_tenant_policy


def user_config(policy: dict) -> UserConfig:
    return UserConfig(
        version="v0.3",
        providers={
            "models": [
                {"name": "cheap"},
                {"name": "premium"},
            ]
        },
        global_={"services": {"tenant_policy": policy}},
    )


def test_tenant_policy_accepts_versioned_provider_and_model_constraints():
    config = user_config(
        {
            "enabled": True,
            "version": "policy-v1",
            "tenant_id_header": "x-authz-tenant-id",
            "require_tenant": True,
            "default": {"allowed_providers": ["mistral", "openai"]},
            "tenants": {
                "economy": {
                    "denied_models": ["premium"],
                    "max_estimated_cost": 0.01,
                }
            },
        }
    )

    assert validate_tenant_policy(config) == []


def test_tenant_policy_rejects_missing_version_and_unknown_model():
    missing_version = validate_tenant_policy(
        user_config({"enabled": True, "default": {}})
    )
    assert missing_version

    unknown_model = validate_tenant_policy(
        user_config(
            {
                "enabled": True,
                "version": "policy-v1",
                "default": {"allowed_models": ["missing"]},
            }
        )
    )
    assert unknown_model
    assert "unknown model 'missing'" in str(unknown_model[0])


def test_tenant_policy_rejects_allow_deny_overlap():
    errors = validate_tenant_policy(
        user_config(
            {
                "enabled": True,
                "version": "policy-v1",
                "default": {
                    "allowed_providers": ["OpenAI"],
                    "denied_providers": ["openai"],
                },
            }
        )
    )

    assert errors
    assert "both allowed and denied" in str(errors[0])


def test_tenant_policy_requires_pricing_for_cost_cap():
    errors = validate_tenant_policy(
        user_config(
            {
                "enabled": True,
                "version": "policy-v1",
                "require_pricing": False,
                "require_current_pricing": False,
                "default": {"max_estimated_cost": 0.01},
            }
        )
    )

    assert errors
    assert "require_pricing must be true" in str(errors[0])
