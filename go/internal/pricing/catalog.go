package pricing

import (
	"context"
	"fmt"
	"strings"

	cloudbilling "google.golang.org/api/cloudbilling/v1"
)

const computeServiceName = "services/6F81-5844-456A"

type Catalog interface {
	ListComputeSKUs(ctx context.Context, currency string) ([]*cloudbilling.Sku, error)
}

type CloudCatalog struct {
	service *cloudbilling.APIService
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

	call := c.service.Services.Skus.List(computeServiceName).CurrencyCode(curr).PageSize(5000)
	var out []*cloudbilling.Sku
	if err := call.Pages(ctx, func(resp *cloudbilling.ListSkusResponse) error {
		out = append(out, resp.Skus...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list compute skus: %w", err)
	}
	return out, nil
}
