package testcases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/vllm-project/semantic-router/e2e/pkg/fixtures"
	pkgtestcases "github.com/vllm-project/semantic-router/e2e/pkg/testcases"
	"k8s.io/client-go/kubernetes"
)

const routingDecisionContractVersion = "vllm-sr/routing-decision/v1alpha1"

func init() {
	pkgtestcases.Register("apiserver-routing-decision", pkgtestcases.TestCase{
		Description: "Verify privacy-safe, non-forwarding routing decisions against the live router runtime",
		Tags:        []string{"kubernetes", "apiserver", "routing", "api", "privacy"},
		Fn:          testAPIServerRoutingDecision,
	})
}

type routingDecisionDocument struct {
	SchemaVersion string `json:"schema_version"`
	DryRun        bool   `json:"dry_run"`
	Route         struct {
		Decision string `json:"decision"`
	} `json:"route"`
	Selection struct {
		Status string `json:"status"`
	} `json:"selection"`
}

func testAPIServerRoutingDecision(
	ctx context.Context,
	client *kubernetes.Clientset,
	opts pkgtestcases.TestCaseOptions,
) error {
	const privatePrompt = "PRIVATE_E2E_ROUTING_DECISION_PROMPT"
	session, err := fixtures.OpenRouterAPISession(ctx, client, opts)
	if err != nil {
		return err
	}
	defer session.Close()

	body, err := json.Marshal(map[string]string{"text": privatePrompt})
	if err != nil {
		return fmt.Errorf("marshal routing decision payload: %w", err)
	}
	response, err := postJSON(
		ctx,
		session.HTTPClient(30*time.Second),
		http.MethodPost,
		session.URL("/api/v1/route/evaluate"),
		body,
	)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected routing decision status 200, got %d: %s", response.StatusCode, string(response.Body))
	}
	if bytes.Contains(response.Body, []byte(privatePrompt)) || bytes.Contains(response.Body, []byte("original_text")) {
		return fmt.Errorf("routing decision response exposed private request content")
	}

	var document routingDecisionDocument
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return fmt.Errorf("decode routing decision response: %w", err)
	}
	if document.SchemaVersion != routingDecisionContractVersion || !document.DryRun {
		return fmt.Errorf("unexpected routing decision contract: version=%q dry_run=%t", document.SchemaVersion, document.DryRun)
	}
	if document.Route.Decision == "" {
		return fmt.Errorf("routing decision response omitted the selected decision")
	}
	if opts.SetDetails != nil {
		opts.SetDetails(map[string]interface{}{
			"schema_version": document.SchemaVersion,
			"decision":       document.Route.Decision,
			"selection":      document.Selection.Status,
		})
	}
	return nil
}
