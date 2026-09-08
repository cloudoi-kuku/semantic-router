"""Small product-neutral client for the stable routing-decision API."""

from __future__ import annotations

from typing import Any, Mapping, Optional

import requests
from pydantic import BaseModel, ConfigDict, Field, ValidationError

ROUTING_DECISION_SCHEMA_V1 = "vllm-sr/routing-decision/v1"


class RouteSelection(BaseModel):
    model_config = ConfigDict(extra="allow")

    status: str = ""
    method: str = ""
    selected_model: str = ""
    candidate_models: list[str] = Field(default_factory=list)
    reason: str = ""


class RouteIdentity(BaseModel):
    model_config = ConfigDict(extra="allow")

    recipe: str = ""
    decision: str = ""
    algorithm: str = ""


class TenantPolicyEvidence(BaseModel):
    model_config = ConfigDict(extra="allow")

    contract_version: str = ""
    policy_version: str = ""
    status: str = ""
    source: str = ""
    tenant_present: bool = False
    require_tenant: bool = False


class RoutingDecisionResponse(BaseModel):
    """Stable response fields; additive server fields remain compatible."""

    model_config = ConfigDict(extra="allow")

    schema_version: str
    dry_run: bool
    route: RouteIdentity
    selection: RouteSelection
    tenant_policy: Optional[TenantPolicyEvidence] = None


class RoutingAPIError(RuntimeError):
    """A content-free routing API failure."""

    def __init__(self, status_code: int, code: str = "ROUTING_API_ERROR"):
        self.status_code = status_code
        self.code = code
        super().__init__(f"routing API failed with HTTP {status_code} ({code})")


class RoutingCompatibilityError(RuntimeError):
    """Raised when the server does not return the stable v1 contract."""


class SemanticRouterClient:
    """Client for non-generating routing decisions.

    Tenant identity is carried only in a configured trusted header. Calling
    products should place this client behind an authenticating gateway that
    strips caller-supplied identity headers before injecting verified values.
    """

    def __init__(
        self,
        base_url: str,
        *,
        token: str | None = None,
        tenant_id_header: str = "x-authz-tenant-id",
        timeout: float = 10.0,
        session: requests.Session | None = None,
    ):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.tenant_id_header = tenant_id_header
        self.timeout = timeout
        self.session = session or requests.Session()

    def evaluate(
        self,
        payload: Mapping[str, Any],
        *,
        tenant_id: str | None = None,
        trace: bool = False,
    ) -> RoutingDecisionResponse:
        headers = {"content-type": "application/json"}
        if self.token:
            headers["authorization"] = f"Bearer {self.token}"
        if tenant_id:
            headers[self.tenant_id_header] = tenant_id
        response = self.session.post(
            f"{self.base_url}/v1/route/evaluate",
            params={"trace": "true"} if trace else None,
            headers=headers,
            json=dict(payload),
            timeout=self.timeout,
        )
        if response.status_code >= 400:
            raise RoutingAPIError(response.status_code, _response_error_code(response))
        try:
            decision = RoutingDecisionResponse.model_validate(response.json())
        except (ValueError, TypeError, ValidationError) as exc:
            raise RoutingCompatibilityError(
                "routing API returned an invalid response contract"
            ) from None
        if decision.schema_version != ROUTING_DECISION_SCHEMA_V1:
            raise RoutingCompatibilityError(
                "routing API returned an incompatible schema version"
            )
        if not decision.dry_run:
            raise RoutingCompatibilityError(
                "routing decision endpoint returned a non-dry-run response"
            )
        return decision


def _response_error_code(response: requests.Response) -> str:
    try:
        body = response.json()
    except (ValueError, TypeError):
        return "ROUTING_API_ERROR"
    if not isinstance(body, dict):
        return "ROUTING_API_ERROR"
    error = body.get("error")
    if not isinstance(error, dict):
        return "ROUTING_API_ERROR"
    code = error.get("code")
    return code if isinstance(code, str) and code else "ROUTING_API_ERROR"
