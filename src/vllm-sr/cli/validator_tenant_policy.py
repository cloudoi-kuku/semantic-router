"""Validation for the canonical global tenant-routing policy."""

from pydantic import ValidationError as PydanticValidationError

from cli.models import TenantPolicyConfig, UserConfig
from cli.validation_error import ValidationError


def validate_tenant_policy(config: UserConfig) -> list[ValidationError]:
    global_config = config.global_ or {}
    if not isinstance(global_config, dict):
        return []
    services = global_config.get("services") or {}
    if not isinstance(services, dict):
        return []
    raw_policy = services.get("tenant_policy")
    if raw_policy is None:
        return []
    try:
        policy = TenantPolicyConfig.model_validate(raw_policy)
    except PydanticValidationError as exc:
        return [ValidationError(str(exc), field="global.services.tenant_policy")]

    configured_models = {model.name for model in config.providers.models}
    errors: list[ValidationError] = []
    policies = [("default", policy.default), *policy.tenants.items()]
    for name, tenant in policies:
        for model in [*tenant.allowed_models, *tenant.denied_models]:
            if model not in configured_models:
                errors.append(
                    ValidationError(
                        f"unknown model '{model}'",
                        field=f"global.services.tenant_policy.{name}",
                    )
                )
    return errors
