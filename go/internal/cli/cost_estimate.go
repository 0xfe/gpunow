package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"

	"gpunow/internal/gcp"
	"gpunow/internal/pricing"
)

func estimateClusterCreateCost(ctx context.Context, state *State, compute gcp.Compute, numInstances int, refresh bool) error {
	machineType := strings.TrimSpace(state.Config.Instance.MachineType)
	mt, err := compute.GetMachineType(ctx, &computepb.GetMachineTypeRequest{
		Project:     state.Config.Project.ID,
		Zone:        state.Config.Project.Zone,
		MachineType: machineType,
	})
	if err != nil {
		return fmt.Errorf("load machine type %s for cost estimation: %w", machineType, err)
	}

	vcpu := mt.GetGuestCpus()
	memoryMB := mt.GetMemoryMb()
	if vcpu <= 0 || memoryMB <= 0 {
		return fmt.Errorf("machine type %s did not return usable CPU/RAM specs", machineType)
	}

	cachePath := filepath.Join(state.Home.StateDir, "pricing-cache.json")
	cacheStore := pricing.NewCacheStore(cachePath)
	catalog, err := pricing.NewCloudCatalog(ctx)
	if err != nil {
		return err
	}

	estimator := pricing.NewEstimator(cacheStore, catalog)
	result, err := estimator.Estimate(ctx, pricing.Request{
		Currency:          "USD",
		Zone:              state.Config.Project.Zone,
		MachineType:       machineType,
		VCPU:              int64(vcpu),
		MemoryMB:          int64(memoryMB),
		ProvisioningModel: state.Config.Instance.ProvisioningModel,
		GPUType:           state.Config.GPU.Type,
		GPUCount:          state.Config.GPU.Count,
		DiskType:          state.Config.Disk.Type,
		DiskSizeGB:        state.Config.Disk.SizeGB,
		NumInstances:      numInstances,
		MaxRunHours:       state.Config.Instance.MaxRunHours,
		Refresh:           refresh,
	})
	if err != nil {
		return fmt.Errorf("estimate cost: %w", err)
	}

	state.UI.Heading("Cost estimate")
	state.UI.Infof("Instances: %d | Machine: %s | Zone: %s", numInstances, machineType, state.Config.Project.Zone)
	for _, component := range result.Components {
		state.UI.Infof("%s: $%.6f per %s", component.Name, component.UnitPrice, component.UsageUnit)
		state.UI.InfofIndent(1, "Quantity: %.2f %s per instance", component.QuantityPerInstance, component.QuantityUnit)
		state.UI.InfofIndent(1, "Total: $%.4f/hour | $%.4f for %d hours", component.CostPerHour, component.CostForMaxRun, result.MaxRunHours)
	}
	state.UI.Successf("Estimated total (%s): $%.4f/hour", result.Currency, result.TotalPerHour)
	state.UI.Successf("Estimated max-run total (%d hours): $%.4f", result.MaxRunHours, result.TotalForMaxRun)
	if result.FetchedSKUs {
		state.UI.Infof("Pricing source: Cloud Billing API (cache updated at %s)", result.CachePath)
	} else {
		state.UI.Infof("Pricing source: local cache (%s)", result.CachePath)
	}
	state.UI.Infof("Estimate excludes egress, discounts, credits, taxes, and license premiums.")
	fmt.Fprintln(state.UI.Out)
	return nil
}
