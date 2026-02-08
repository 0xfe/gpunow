package pricing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	cloudbilling "google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/googleapi"
)

const computeServiceName = "services/6F81-5844-456A"

type Catalog interface {
	ListComputeSKUs(ctx context.Context, currency string) ([]*cloudbilling.Sku, error)
}

type CloudCatalog struct {
	service      *cloudbilling.APIService
	listObserver func(action string, resource string) func()
	mu           sync.RWMutex
}

func NewCloudCatalog(ctx context.Context) (*CloudCatalog, error) {
	svc, err := cloudbilling.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("init cloud billing service: %w", err)
	}
	return &CloudCatalog{service: svc}, nil
}

func (c *CloudCatalog) ListComputeSKUs(ctx context.Context, currency string) ([]*cloudbilling.Sku, error) {
	if c == nil || c.service == nil {
		return nil, fmt.Errorf("cloud billing service is not initialized")
	}
	curr := strings.ToUpper(strings.TrimSpace(currency))
	if curr == "" {
		curr = "USD"
	}

	stopObserver := c.startListObserver("cloudbilling.services.skus.list", fmt.Sprintf("%s?currencyCode=%s", computeServiceName, curr))
	if stopObserver != nil {
		defer stopObserver()
	}

	call := c.service.Services.Skus.List(computeServiceName).CurrencyCode(curr).PageSize(5000)
	var out []*cloudbilling.Sku
	if err := call.Pages(ctx, func(resp *cloudbilling.ListSkusResponse) error {
		out = append(out, resp.Skus...)
		return nil
	}); err != nil {
		return nil, formatCatalogListError(err)
	}
	return out, nil
}

func (c *CloudCatalog) SetListObserver(observer func(action string, resource string) func()) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listObserver = observer
}

func (c *CloudCatalog) startListObserver(action string, resource string) func() {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	observer := c.listObserver
	c.mu.RUnlock()
	if observer == nil {
		return nil
	}
	return observer(action, resource)
}

func formatCatalogListError(err error) error {
	if err == nil {
		return nil
	}
	info, ok := parseServiceDisabledError(err)
	if !ok {
		return fmt.Errorf("list compute skus: %w", err)
	}

	service := info.service
	if service == "" {
		service = "cloudbilling.googleapis.com"
	}
	message := info.messageVerbatim
	if strings.TrimSpace(message) == "" {
		message = "Cloud Billing API is disabled for this project."
	}

	lines := []string{
		message,
		fmt.Sprintf("To use `gpunow create --estimate-cost`, enable `%s` and retry.", service),
	}
	if info.projectID != "" {
		lines = append(lines, fmt.Sprintf("Run: gcloud services enable %s --project %s", service, info.projectID))
	} else {
		lines = append(lines, fmt.Sprintf("Run: gcloud services enable %s --project <your-project-id>", service))
	}
	if info.url != "" {
		lines = append(lines, fmt.Sprintf("Console: %s", info.url))
	}
	lines = append(lines, "If you enabled it recently, wait a few minutes for propagation and retry.")
	return errors.New(strings.Join(lines, "\n"))
}

type serviceDisabledInfo struct {
	projectID       string
	service         string
	url             string
	messageVerbatim string
}

func parseServiceDisabledError(err error) (serviceDisabledInfo, bool) {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return serviceDisabledInfo{}, false
	}
	info := serviceDisabledInfo{
		messageVerbatim: gerr.Message,
	}
	reason := ""
	for _, detail := range gerr.Details {
		detailMap, ok := detail.(map[string]any)
		if !ok {
			continue
		}
		typeURL := strings.TrimSpace(stringValue(detailMap["@type"]))
		switch {
		case strings.HasSuffix(typeURL, "google.rpc.ErrorInfo"):
			reason = strings.ToUpper(strings.TrimSpace(stringValue(detailMap["reason"])))
			metadata := mapValue(detailMap["metadata"])
			if activationURL := strings.TrimSpace(stringValue(metadata["activationUrl"])); activationURL != "" {
				info.url = activationURL
			}
			if service := strings.TrimSpace(stringValue(metadata["service"])); service != "" {
				info.service = service
			}
			consumer := strings.TrimSpace(stringValue(metadata["consumer"]))
			if strings.HasPrefix(consumer, "projects/") {
				info.projectID = strings.TrimPrefix(consumer, "projects/")
			}
		case strings.HasSuffix(typeURL, "google.rpc.LocalizedMessage"):
			if message := stringValue(detailMap["message"]); strings.TrimSpace(message) != "" {
				info.messageVerbatim = message
			}
		case strings.HasSuffix(typeURL, "google.rpc.Help"):
			if info.url != "" {
				continue
			}
			links, ok := detailMap["links"].([]any)
			if !ok {
				continue
			}
			for _, link := range links {
				linkMap, ok := link.(map[string]any)
				if !ok {
					continue
				}
				if url := strings.TrimSpace(stringValue(linkMap["url"])); url != "" {
					info.url = url
					break
				}
			}
		}
	}

	if reason != "SERVICE_DISABLED" {
		return serviceDisabledInfo{}, false
	}
	return info, true
}

func stringValue(in any) string {
	if in == nil {
		return ""
	}
	value, ok := in.(string)
	if !ok {
		return ""
	}
	return value
}

func mapValue(in any) map[string]any {
	if in == nil {
		return nil
	}
	value, ok := in.(map[string]any)
	if !ok {
		return nil
	}
	return value
}
