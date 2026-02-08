package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"

	"gpunow/internal/gcp"
	"gpunow/internal/pricing"
	appstate "gpunow/internal/state"
)

func estimateClusterCreateCost(ctx context.Context, state *State, compute gcp.Compute, numInstances int, refresh bool, clusterConfig appstate.ClusterConfig) error {
	machineType := strings.TrimSpace(state.Config.Instance.MachineType)
	if override := strings.TrimSpace(clusterConfig.GCPMachineType); override != "" {
		machineType = override
	}
	split := state.UI.StartLiveSplit()
	if split != nil {
		defer split.Stop()
	}
	progress := state.UI.TaskList("Estimating", []string{
		fmt.Sprintf("Loading machine type %s", machineType),
		"Resolving pricing data",
	})
	defer progress.Stop()

	project := state.Config.Project.ID
	zone := state.Config.Project.Zone
	machineTypeCall := state.UI.APICall("compute.machineTypes.get", gcp.ZoneResource(project, zone, "machineTypes", machineType), "")
	mt, err := compute.GetMachineType(ctx, &computepb.GetMachineTypeRequest{
		Project:     project,
		Zone:        zone,
		MachineType: machineType,
	})
	machineTypeCall.Stop()
	if err != nil {
		progress.MarkWarning(0, fmt.Sprintf("Failed machineTypes/%s", machineType))
		return fmt.Errorf("load machine type %s for cost estimation: %w", machineType, err)
	}
	progress.MarkDone(0, fmt.Sprintf("Loaded machineTypes/%s", machineType))

	vcpu := mt.GetGuestCpus()
	memoryMB := mt.GetMemoryMb()
	if vcpu <= 0 || memoryMB <= 0 {
		return fmt.Errorf("machine type %s did not return usable CPU/RAM specs", machineType)
	}
	gpuType, gpuCount, err := machineTypeGPU(mt)
	if err != nil {
		return err
	}

	maxRunHours := state.Config.Instance.MaxRunHours
	if clusterConfig.GCPMaxRunHours > 0 {
		maxRunHours = clusterConfig.GCPMaxRunHours
	}
	diskSizeGB := state.Config.Disk.SizeGB
	if clusterConfig.GCPDiskSizeGB > 0 {
		diskSizeGB = clusterConfig.GCPDiskSizeGB
	}

	cachePath := filepath.Join(state.Home.StateDir, "pricing-cache.json")
	cacheStore := pricing.NewCacheStore(cachePath)
	catalog, err := pricing.NewCloudCatalog(ctx)
	if err != nil {
		progress.MarkWarning(1, "Failed to initialize Cloud Billing API client")
		return err
	}
	catalog.SetListObserver(func(action string, resource string) func() {
		call := state.UI.APICall(action, resource, "")
		return func() {
			call.Stop()
		}
	})

	estimator := pricing.NewEstimator(cacheStore, catalog)
	result, err := estimator.Estimate(ctx, pricing.Request{
		Currency:          "USD",
		Zone:              zone,
		MachineType:       machineType,
		VCPU:              int64(vcpu),
		MemoryMB:          int64(memoryMB),
		ProvisioningModel: state.Config.Instance.ProvisioningModel,
		GPUType:           gpuType,
		GPUCount:          gpuCount,
		DiskType:          state.Config.Disk.Type,
		DiskSizeGB:        diskSizeGB,
		NumInstances:      numInstances,
		MaxRunHours:       maxRunHours,
		Refresh:           refresh,
	})
	if err != nil {
		progress.MarkWarning(1, "Failed pricing lookup")
		return fmt.Errorf("estimate cost: %w", err)
	}
	if result.FetchedSKUs {
		progress.MarkDone(1, "Loaded pricing catalog from Cloud Billing API")
	} else {
		progress.MarkDone(1, "Using cached pricing data")
	}

	state.UI.Heading("Cost estimate")
	state.UI.Infof("Instances: %d | Machine: %s | Zone: %s", numInstances, machineType, zone)
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

func machineTypeGPU(mt *computepb.MachineType) (string, int, error) {
	if mt == nil {
		return "", 0, nil
	}
	gpuType := ""
	gpuCount := 0
	for _, accel := range mt.GetAccelerators() {
		if accel == nil {
			continue
		}
		accelType := strings.TrimSpace(accel.GetGuestAcceleratorType())
		accelCount := int(accel.GetGuestAcceleratorCount())
		if accelCount <= 0 {
			continue
		}
		if gpuType == "" {
			gpuType = accelType
		}
		if accelType != "" && gpuType != accelType {
			return "", 0, fmt.Errorf("machine type includes multiple GPU types (%s and %s); unsupported for pricing estimation", gpuType, accelType)
		}
		gpuCount += accelCount
	}
	return gpuType, gpuCount, nil
}
