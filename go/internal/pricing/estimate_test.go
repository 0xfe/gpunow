package pricing

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	cloudbilling "google.golang.org/api/cloudbilling/v1"
)

type fakeCatalog struct {
	skus  []*cloudbilling.Sku
	err   error
	calls int
}

func (f *fakeCatalog) ListComputeSKUs(_ context.Context, _ string) ([]*cloudbilling.Sku, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.skus, nil
}

func TestEstimatorUsesCatalogAndCache(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core", "G2 Instance Core running in us-east1", "Compute", "CPU", "Preemptible", "h", 0.05, []string{"us-east1"}),
			testSKU("ram", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Preemptible", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			testSKU("disk", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"}),
		},
	}
	estimator := NewEstimator(cache, catalog)

	result, err := estimator.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      2,
		MaxRunHours:       12,
	})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if !result.FetchedSKUs {
		t.Fatalf("expected fetched SKUs")
	}
	if catalog.calls != 1 {
		t.Fatalf("catalog calls mismatch: got=%d want=1", catalog.calls)
	}

	wantPerHour := (16*0.05 + 64*0.01 + 1*0.30 + (200*0.04)/730.0) * 2
	if !closeEnough(result.TotalPerHour, wantPerHour) {
		t.Fatalf("total per hour mismatch: got=%.8f want=%.8f", result.TotalPerHour, wantPerHour)
	}
	if !closeEnough(result.TotalForMaxRun, wantPerHour*12) {
		t.Fatalf("total for max run mismatch: got=%.8f want=%.8f", result.TotalForMaxRun, wantPerHour*12)
	}

	cachedCatalog := &fakeCatalog{err: context.Canceled}
	estimatorCached := NewEstimator(cache, cachedCatalog)
	result2, err := estimatorCached.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      2,
		MaxRunHours:       12,
	})
	if err != nil {
		t.Fatalf("estimate from cache: %v", err)
	}
	if result2.FetchedSKUs {
		t.Fatalf("expected cache-only estimate")
	}
	if cachedCatalog.calls != 0 {
		t.Fatalf("expected no catalog calls when cache is complete")
	}
}

func TestEstimatorRefreshBypassesCache(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core", "G2 Instance Core running in us-east1", "Compute", "CPU", "Preemptible", "h", 0.05, []string{"us-east1"}),
			testSKU("ram", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Preemptible", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			testSKU("disk", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"}),
		},
	}
	estimator := NewEstimator(cache, catalog)

	req := Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      1,
		MaxRunHours:       12,
	}
	if _, err := estimator.Estimate(context.Background(), req); err != nil {
		t.Fatalf("first estimate: %v", err)
	}
	req.Refresh = true
	if _, err := estimator.Estimate(context.Background(), req); err != nil {
		t.Fatalf("refresh estimate: %v", err)
	}
	if catalog.calls != 2 {
		t.Fatalf("catalog calls mismatch: got=%d want=2", catalog.calls)
	}
}

func TestEstimatorAmbiguousSKUsFails(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core-a", "G2 Instance Core running in us-east1", "Compute", "CPU", "Preemptible", "h", 0.05, []string{"us-east1"}),
			testSKU("core-b", "G2 Instance Core running in us-east1", "Compute", "CPU", "Preemptible", "h", 0.05, []string{"us-east1"}),
			testSKU("ram", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Preemptible", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			testSKU("disk", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"}),
		},
	}
	estimator := NewEstimator(cache, catalog)

	_, err := estimator.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      1,
		MaxRunHours:       12,
	})
	if err == nil {
		t.Fatalf("expected ambiguous sku error")
	}
}

func TestEstimatorPrefersSpotOverPreemptibleWhenBothMatch(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core-preemptible", "G2 Instance Core running in us-east1", "Compute", "CPU", "Preemptible", "h", 0.05, []string{"us-east1"}),
			testSKU("core-spot", "G2 Instance Core running in us-east1", "Compute", "CPU", "Spot", "h", 0.055, []string{"us-east1"}),
			testSKU("ram", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Spot", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			testSKU("disk", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"}),
		},
	}
	estimator := NewEstimator(cache, catalog)

	result, err := estimator.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      1,
		MaxRunHours:       12,
	})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if len(result.Components) == 0 {
		t.Fatalf("expected pricing components")
	}
	if result.Components[0].SKUID != "core-spot" {
		t.Fatalf("expected spot sku, got %s", result.Components[0].SKUID)
	}
}

func TestEstimatorExcludesCustomSKUForPredefinedMachineTypes(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core-custom", "G2 Custom Instance Core running in us-east1", "Compute", "CPU", "Spot", "h", 0.04, []string{"us-east1"}),
			testSKU("core-predefined", "G2 Instance Core running in us-east1", "Compute", "CPU", "Spot", "h", 0.05, []string{"us-east1"}),
			testSKU("ram-custom", "G2 Custom Instance Ram running in us-east1", "Compute", "RAM", "Spot", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("ram-predefined", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Spot", "GiBy.h", 0.02, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			testSKU("disk", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"}),
		},
	}
	estimator := NewEstimator(cache, catalog)

	result, err := estimator.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      1,
		MaxRunHours:       12,
	})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	var coreSKU, ramSKU string
	for _, component := range result.Components {
		if component.Name == "vCPU" {
			coreSKU = component.SKUID
		}
		if component.Name == "RAM" {
			ramSKU = component.SKUID
		}
	}
	if coreSKU != "core-predefined" {
		t.Fatalf("expected predefined core sku, got %s", coreSKU)
	}
	if ramSKU != "ram-predefined" {
		t.Fatalf("expected predefined ram sku, got %s", ramSKU)
	}
}

func TestEstimatorPrefersZonalPDStandardOverRegional(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core", "G2 Instance Core running in us-east1", "Compute", "CPU", "Spot", "h", 0.05, []string{"us-east1"}),
			testSKU("ram", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Spot", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			testSKU("disk-zonal", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"}),
			testSKU("disk-regional", "Regional Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.08, []string{"us-east1"}),
		},
	}
	estimator := NewEstimator(cache, catalog)

	result, err := estimator.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      1,
		MaxRunHours:       12,
	})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	diskSKU := ""
	for _, component := range result.Components {
		if component.Name == "Disk" {
			diskSKU = component.SKUID
			break
		}
	}
	if diskSKU != "disk-zonal" {
		t.Fatalf("expected zonal disk sku, got %s", diskSKU)
	}
}

func TestEstimatorUsesFirstPositiveTierPrice(t *testing.T) {
	tmp := t.TempDir()
	cache := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))
	disk := testSKU("disk", "Storage PD Capacity in us-east1", "Storage", "PDStandard", "OnDemand", "GiBy.mo", 0.04, []string{"us-east1"})
	disk.PricingInfo[0].PricingExpression.TieredRates = []*cloudbilling.TierRate{
		{
			StartUsageAmount: 0,
			UnitPrice:        money(0),
		},
		{
			StartUsageAmount: 30,
			UnitPrice:        money(0.04),
		},
	}
	catalog := &fakeCatalog{
		skus: []*cloudbilling.Sku{
			testSKU("core", "G2 Instance Core running in us-east1", "Compute", "CPU", "Spot", "h", 0.05, []string{"us-east1"}),
			testSKU("ram", "G2 Instance Ram running in us-east1", "Compute", "RAM", "Spot", "GiBy.h", 0.01, []string{"us-east1"}),
			testSKU("gpu", "NVIDIA L4 GPU attached to Spot VMs running in us-east1", "Compute", "GPU", "Spot", "h", 0.30, []string{"us-east1"}),
			disk,
		},
	}
	estimator := NewEstimator(cache, catalog)

	result, err := estimator.Estimate(context.Background(), Request{
		Currency:          "USD",
		Zone:              "us-east1-d",
		MachineType:       "g2-standard-16",
		VCPU:              16,
		MemoryMB:          65536,
		ProvisioningModel: "SPOT",
		GPUType:           "nvidia-l4",
		GPUCount:          1,
		DiskType:          "pd-standard",
		DiskSizeGB:        200,
		NumInstances:      1,
		MaxRunHours:       12,
	})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	diskUnitPrice := -1.0
	for _, component := range result.Components {
		if component.Name == "Disk" {
			diskUnitPrice = component.UnitPrice
			break
		}
	}
	if !closeEnough(diskUnitPrice, 0.04) {
		t.Fatalf("expected disk unit price 0.04, got %.8f", diskUnitPrice)
	}
}

func testSKU(id, desc, family, group, usage, unit string, price float64, regions []string) *cloudbilling.Sku {
	return &cloudbilling.Sku{
		Name:        "services/6F81-5844-456A/skus/" + id,
		SkuId:       id,
		Description: desc,
		Category: &cloudbilling.Category{
			ResourceFamily: family,
			ResourceGroup:  group,
			UsageType:      usage,
		},
		ServiceRegions: regions,
		PricingInfo: []*cloudbilling.PricingInfo{
			{
				EffectiveTime: "2026-02-07T00:00:00Z",
				PricingExpression: &cloudbilling.PricingExpression{
					UsageUnit: unit,
					TieredRates: []*cloudbilling.TierRate{
						{
							StartUsageAmount: 0,
							UnitPrice:        money(price),
						},
					},
				},
			},
		},
	}
}

func money(value float64) *cloudbilling.Money {
	units := int64(value)
	nanos := int64(math.Round((value - float64(units)) * 1e9))
	return &cloudbilling.Money{
		CurrencyCode: "USD",
		Units:        units,
		Nanos:        nanos,
	}
}

func closeEnough(a, b float64) bool {
	const epsilon = 1e-6
	return math.Abs(a-b) <= epsilon
}
