#!/usr/bin/env python3
"""Gate Niffy coding routes on objective validation and trajectory cost."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

SCHEMA_VERSION = "niffy/code-evaluation/v1"
TIERS = {"economical": 0, "general": 1, "reasoning": 2}
CHECKS = {"patch", "build", "test", "review"}
STATUSES = {"passed", "failed", "not_run"}
INITIAL_TRIGGERS = {"initial", "classified_complex"}


class EvaluationError(ValueError):
    """Raised when an evaluation ledger violates the public contract."""


def _number(value: Any, field: str) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)) or value < 0:
        raise EvaluationError(f"{field} must be a non-negative number")
    return float(value)


def _threshold(value: Any, field: str, maximum: float | None = None) -> float:
    result = _number(value, field)
    if maximum is not None and result > maximum:
        raise EvaluationError(f"{field} must be at most {maximum}")
    return result


def _validate_attempt(task_id: str, index: int, raw: Any) -> dict[str, Any]:
    field = f"tasks[{task_id}].attempts[{index}]"
    if not isinstance(raw, dict):
        raise EvaluationError(f"{field} must be an object")
    tier = str(raw.get("tier") or "")
    if tier not in TIERS:
        raise EvaluationError(f"{field}.tier must be one of {sorted(TIERS)}")
    trigger = str(raw.get("trigger") or "")
    validation = raw.get("validation")
    if not isinstance(validation, dict) or not validation:
        raise EvaluationError(f"{field}.validation must be a non-empty object")
    normalized_validation: dict[str, str] = {}
    for check, status in validation.items():
        if check not in CHECKS or status not in STATUSES:
            raise EvaluationError(
                f"{field}.validation supports {sorted(CHECKS)} with statuses {sorted(STATUSES)}"
            )
        normalized_validation[check] = status
    return {
        "tier": tier,
        "trigger": trigger,
        "cost_usd": _number(raw.get("cost_usd"), f"{field}.cost_usd"),
        "validation": normalized_validation,
    }


def _validate_escalations(task_id: str, attempts: list[dict[str, Any]]) -> list[str]:
    violations: list[str] = []
    if attempts[0]["trigger"] not in INITIAL_TRIGGERS:
        violations.append(
            f"{task_id}: first attempt has invalid trigger {attempts[0]['trigger']!r}"
        )
    for index in range(1, len(attempts)):
        previous = attempts[index - 1]
        current = attempts[index]
        if TIERS[current["tier"]] <= TIERS[previous["tier"]]:
            violations.append(
                f"{task_id}: attempt {index + 1} does not move to a higher tier"
            )
            continue
        failed_checks = {
            f"{check}_failed"
            for check, status in previous["validation"].items()
            if status == "failed"
        }
        if current["trigger"] not in failed_checks:
            violations.append(
                f"{task_id}: attempt {index + 1} trigger {current['trigger']!r} "
                "is not backed by the previous attempt's failed validation"
            )
    return violations


def _resolve_thresholds(document: dict[str, Any]) -> tuple[float, float, float]:
    thresholds = document.get("thresholds")
    if not isinstance(thresholds, dict):
        raise EvaluationError("thresholds must be an object")
    min_validated_rate = _threshold(
        thresholds.get("min_validated_rate"), "thresholds.min_validated_rate", 1.0
    )
    max_cost = _threshold(
        thresholds.get("max_cost_per_validated_outcome_usd"),
        "thresholds.max_cost_per_validated_outcome_usd",
    )
    min_savings = _threshold(
        thresholds.get("min_savings_vs_baseline_rate"),
        "thresholds.min_savings_vs_baseline_rate",
        1.0,
    )
    return min_validated_rate, max_cost, min_savings


def _evaluate_task(
    raw_task: Any, index: int, seen: set[str]
) -> tuple[dict[str, Any], list[str]]:
    if not isinstance(raw_task, dict):
        raise EvaluationError(f"tasks[{index}] must be an object")
    task_id = str(raw_task.get("id") or "").strip()
    if not task_id or task_id in seen:
        raise EvaluationError(f"tasks[{index}].id must be non-empty and unique")
    seen.add(task_id)
    required = raw_task.get("required_checks")
    if (
        not isinstance(required, list)
        or not required
        or any(check not in CHECKS for check in required)
        or len(set(required)) != len(required)
    ):
        raise EvaluationError(
            f"tasks[{task_id}].required_checks must be unique values from {sorted(CHECKS)}"
        )
    raw_attempts = raw_task.get("attempts")
    if not isinstance(raw_attempts, list) or not raw_attempts:
        raise EvaluationError(f"tasks[{task_id}].attempts must be a non-empty array")
    attempts = [
        _validate_attempt(task_id, attempt_index, attempt)
        for attempt_index, attempt in enumerate(raw_attempts)
    ]
    task_cost = sum(attempt["cost_usd"] for attempt in attempts)
    task_baseline = _number(
        raw_task.get("baseline_cost_usd"), f"tasks[{task_id}].baseline_cost_usd"
    )
    final_validation = attempts[-1]["validation"]
    return (
        {
            "id": task_id,
            "validated": all(
                final_validation.get(check) == "passed" for check in required
            ),
            "attempts": len(attempts),
            "terminal_tier": attempts[-1]["tier"],
            "cost_usd": round(task_cost, 8),
            "baseline_cost_usd": round(task_baseline, 8),
        },
        _validate_escalations(task_id, attempts),
    )


def evaluate(document: dict[str, Any]) -> dict[str, Any]:
    if document.get("schema_version") != SCHEMA_VERSION:
        raise EvaluationError(f"schema_version must be {SCHEMA_VERSION!r}")
    min_validated_rate, max_cost, min_savings = _resolve_thresholds(document)
    raw_tasks = document.get("tasks")
    if not isinstance(raw_tasks, list) or not raw_tasks:
        raise EvaluationError("tasks must be a non-empty array")

    task_results: list[dict[str, Any]] = []
    violations: list[str] = []
    total_cost = 0.0
    baseline_cost = 0.0
    seen: set[str] = set()
    for index, raw_task in enumerate(raw_tasks):
        task, task_violations = _evaluate_task(raw_task, index, seen)
        task_results.append(task)
        violations.extend(task_violations)
        total_cost += task["cost_usd"]
        baseline_cost += task["baseline_cost_usd"]

    validated_count = sum(task["validated"] for task in task_results)
    validated_rate = validated_count / len(task_results)
    cost_per_validated = (
        total_cost / validated_count if validated_count else float("inf")
    )
    savings_rate = (
        (baseline_cost - total_cost) / baseline_cost if baseline_cost else float("-inf")
    )
    gates = {
        "objective_escalation": not violations,
        "validated_rate": validated_rate >= min_validated_rate,
        "cost_per_validated_outcome": cost_per_validated <= max_cost,
        "savings_vs_baseline": savings_rate >= min_savings,
    }
    return {
        "schema_version": SCHEMA_VERSION,
        "passed": all(gates.values()),
        "gates": gates,
        "summary": {
            "tasks": len(task_results),
            "validated": validated_count,
            "validated_rate": round(validated_rate, 6),
            "total_cost_usd": round(total_cost, 8),
            "baseline_cost_usd": round(baseline_cost, 8),
            "cost_per_validated_outcome_usd": (
                round(cost_per_validated, 8) if validated_count else None
            ),
            "savings_vs_baseline_rate": (
                round(savings_rate, 6) if baseline_cost else None
            ),
        },
        "violations": violations,
        "tasks": task_results,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("ledger", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    try:
        document = json.loads(args.ledger.read_text(encoding="utf-8"))
        result = evaluate(document)
    except (OSError, json.JSONDecodeError, EvaluationError) as exc:
        print(f"niffy code evaluation: {exc}", file=sys.stderr)
        return 2
    rendered = json.dumps(result, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered, end="")
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
