import unittest

import niffy_code_evaluation as gate


def ledger() -> dict:
    return {
        "schema_version": gate.SCHEMA_VERSION,
        "thresholds": {
            "min_validated_rate": 1.0,
            "max_cost_per_validated_outcome_usd": 0.05,
            "min_savings_vs_baseline_rate": 0.25,
        },
        "tasks": [
            {
                "id": "small-change",
                "required_checks": ["patch", "test"],
                "baseline_cost_usd": 0.08,
                "attempts": [
                    {
                        "tier": "general",
                        "trigger": "initial",
                        "cost_usd": 0.01,
                        "validation": {"patch": "passed", "test": "failed"},
                    },
                    {
                        "tier": "reasoning",
                        "trigger": "test_failed",
                        "cost_usd": 0.03,
                        "validation": {"patch": "passed", "test": "passed"},
                    },
                ],
            }
        ],
    }


class NiffyCodeEvaluationTest(unittest.TestCase):
    def test_passes_valid_evidence_backed_trajectory(self):
        result = gate.evaluate(ledger())
        self.assertTrue(result["passed"])
        self.assertEqual(result["summary"]["cost_per_validated_outcome_usd"], 0.04)
        self.assertEqual(result["summary"]["savings_vs_baseline_rate"], 0.5)

    def test_rejects_self_asserted_escalation_without_failed_check(self):
        value = ledger()
        value["tasks"][0]["attempts"][0]["validation"]["test"] = "passed"
        result = gate.evaluate(value)
        self.assertFalse(result["passed"])
        self.assertFalse(result["gates"]["objective_escalation"])

    def test_counts_failed_trajectory_cost(self):
        value = ledger()
        value["tasks"][0]["attempts"][-1]["validation"]["test"] = "failed"
        result = gate.evaluate(value)
        self.assertFalse(result["passed"])
        self.assertEqual(result["summary"]["validated"], 0)
        self.assertEqual(result["summary"]["total_cost_usd"], 0.04)

    def test_rejects_non_increasing_tier(self):
        value = ledger()
        value["tasks"][0]["attempts"][-1]["tier"] = "general"
        result = gate.evaluate(value)
        self.assertFalse(result["gates"]["objective_escalation"])

    def test_rejects_unknown_validation_check(self):
        value = ledger()
        value["tasks"][0]["attempts"][0]["validation"]["lint"] = "passed"
        with self.assertRaises(gate.EvaluationError):
            gate.evaluate(value)


if __name__ == "__main__":
    unittest.main()
