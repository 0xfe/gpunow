package ssh

import (
	"context"
	"fmt"

	"cloud.google.com/go/compute/apiv1/computepb"

	"gpunow/internal/config"
	"gpunow/internal/gcp"
	"gpunow/internal/target"
)

type ResolvedTarget struct {
	Cluster        string
	Index          int
	Host           string
	MasterPublicIP string
}

func ResolveTarget(ctx context.Context, compute gcp.Compute, cfg *config.Config, raw string) (*ResolvedTarget, error) {
	parsed, err := target.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.IsCluster {
		return ResolveClusterTarget(ctx, compute, cfg, raw)
	}
	return resolveVMTarget(ctx, compute, cfg, parsed.Name)
}

func ResolveClusterTarget(ctx context.Context, compute gcp.Compute, cfg *config.Config, raw string) (*ResolvedTarget, error) {
	parsed, err := target.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !parsed.IsCluster {
		return nil, fmt.Errorf("target must be cluster/index")
	}
	cluster := parsed.Cluster
	inst, err := getInstance(ctx, compute, cfg, parsed.Name)
	if err != nil {
		return nil, err
	}
	publicIP := externalIP(inst)
	if publicIP == "" {
		return nil, fmt.Errorf("instance %s has no external IP", parsed.Name)
	}
	return &ResolvedTarget{Cluster: cluster, Index: parsed.Index, Host: publicIP, MasterPublicIP: publicIP}, nil
}

func resolveVMTarget(ctx context.Context, compute gcp.Compute, cfg *config.Config, name string) (*ResolvedTarget, error) {
	inst, err := getInstance(ctx, compute, cfg, name)
	if err != nil {
		return nil, err
	}
	ip := externalIP(inst)
	if ip == "" {
		return nil, fmt.Errorf("instance has no external IP")
	}
	return &ResolvedTarget{Host: ip, Index: -1}, nil
}

func getInstance(ctx context.Context, compute gcp.Compute, cfg *config.Config, name string) (*computepb.Instance, error) {
	inst, err := compute.GetInstance(ctx, &computepb.GetInstanceRequest{
		Project:  cfg.Project.ID,
		Zone:     cfg.Project.Zone,
		Instance: name,
	})
	if err != nil {
		if gcp.IsNotFound(err) {
			return nil, fmt.Errorf("instance not found: %s", name)
		}
		return nil, err
	}
	return inst, nil
}

func externalIP(inst *computepb.Instance) string {
	if inst == nil || len(inst.GetNetworkInterfaces()) == 0 {
		return ""
	}
	iface := inst.GetNetworkInterfaces()[0]
	if len(iface.GetAccessConfigs()) == 0 {
		return ""
	}
	return iface.GetAccessConfigs()[0].GetNatIP()
}
