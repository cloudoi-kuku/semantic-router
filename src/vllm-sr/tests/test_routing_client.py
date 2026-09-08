from unittest.mock import Mock

import pytest

from vllm_sr import (
    ROUTING_DECISION_SCHEMA_V1,
    RoutingAPIError,
    RoutingCompatibilityError,
    SemanticRouterClient,
)


def response(status_code: int, body: dict) -> Mock:
    result = Mock()
    result.status_code = status_code
    result.json.return_value = body
    return result


def stable_body() -> dict:
    return {
        "schema_version": ROUTING_DECISION_SCHEMA_V1,
        "dry_run": True,
        "route": {"decision": "economical-default-route", "algorithm": "static"},
        "selection": {"status": "selected", "selected_model": "niffy-cheap"},
        "tenant_policy": {
            "contract_version": "vllm-sr/tenant-policy/v1alpha1",
            "policy_version": "policy-v1",
            "status": "applied",
            "source": "tenant",
            "tenant_present": True,
            "require_tenant": True,
        },
    }


def test_client_uses_stable_endpoint_and_trusted_tenant_header():
    session = Mock()
    session.post.return_value = response(200, stable_body())
    client = SemanticRouterClient(
        "http://router:8080/",
        token="management-token",
        tenant_id_header="x-product-tenant",
        timeout=4,
        session=session,
    )

    decision = client.evaluate(
        {"model": "niffy/auto", "text": "hello"},
        tenant_id="tenant-a",
        trace=True,
    )

    assert decision.selection.selected_model == "niffy-cheap"
    session.post.assert_called_once_with(
        "http://router:8080/v1/route/evaluate",
        params={"trace": "true"},
        headers={
            "content-type": "application/json",
            "authorization": "Bearer management-token",
            "x-product-tenant": "tenant-a",
        },
        json={"model": "niffy/auto", "text": "hello"},
        timeout=4,
    )


def test_client_rejects_alpha_or_executing_contracts():
    session = Mock()
    alpha = stable_body()
    alpha["schema_version"] = "vllm-sr/routing-decision/v1alpha1"
    session.post.return_value = response(200, alpha)
    client = SemanticRouterClient("http://router", session=session)

    with pytest.raises(RoutingCompatibilityError):
        client.evaluate({"text": "hello"})


def test_client_error_does_not_include_request_or_server_message():
    session = Mock()
    session.post.return_value = response(
        403,
        {"error": {"code": "FORBIDDEN", "message": "private prompt text"}},
    )
    client = SemanticRouterClient("http://router", session=session)

    with pytest.raises(RoutingAPIError) as captured:
        client.evaluate({"text": "private prompt text"})

    assert str(captured.value) == "routing API failed with HTTP 403 (FORBIDDEN)"


def test_client_rejects_invalid_response_without_echoing_body():
    session = Mock()
    session.post.return_value = response(200, {"private": "prompt text"})
    client = SemanticRouterClient("http://router", session=session)

    with pytest.raises(RoutingCompatibilityError) as captured:
        client.evaluate({"text": "prompt text"})

    assert str(captured.value) == "routing API returned an invalid response contract"
    assert "private prompt text" not in str(captured.value)
