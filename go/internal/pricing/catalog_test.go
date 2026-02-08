package pricing

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestFormatCatalogListErrorServiceDisabled(t *testing.T) {
	message := "Cloud Billing API has not been used in project symbolic-axe-717 before or it is disabled."
	err := &googleapi.Error{
		Code:    403,
		Message: message,
		Details: []any{
			map[string]any{
				"@type":  "type.googleapis.com/google.rpc.ErrorInfo",
				"reason": "SERVICE_DISABLED",
				"metadata": map[string]any{
					"service":       "cloudbilling.googleapis.com",
					"consumer":      "projects/symbolic-axe-717",
					"activationUrl": "https://console.developers.google.com/apis/api/cloudbilling.googleapis.com/overview?project=symbolic-axe-717",
				},
			},
			map[string]any{
				"@type":   "type.googleapis.com/google.rpc.LocalizedMessage",
				"message": message,
			},
		},
	}

	out := formatCatalogListError(err)
	if out == nil {
		t.Fatalf("expected formatted error")
	}
	text := out.Error()
	if !strings.Contains(text, message) {
		t.Fatalf("expected verbatim API message, got:\n%s", text)
	}
	if !strings.Contains(text, "Run: gcloud services enable cloudbilling.googleapis.com --project symbolic-axe-717") {
		t.Fatalf("expected gcloud enable instruction, got:\n%s", text)
	}
	if !strings.Contains(text, "Console: https://console.developers.google.com/apis/api/cloudbilling.googleapis.com/overview?project=symbolic-axe-717") {
		t.Fatalf("expected console link, got:\n%s", text)
	}
}

func TestFormatCatalogListErrorFallback(t *testing.T) {
	in := errors.New("boom")
	out := formatCatalogListError(in)
	if out == nil {
		t.Fatalf("expected wrapped error")
	}
	if !strings.Contains(out.Error(), "list compute skus") {
		t.Fatalf("expected list compute skus prefix, got: %s", out.Error())
	}
}
