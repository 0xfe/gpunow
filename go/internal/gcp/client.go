package gcp

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
)

type Client struct {
	Instances    *compute.InstancesClient
	Disks        *compute.DisksClient
	Firewalls    *compute.FirewallsClient
	Networks     *compute.NetworksClient
	Subnetworks  *compute.SubnetworksClient
	MachineTypes *compute.MachineTypesClient
}

func New(ctx context.Context) (*Client, error) {
	instances, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("instances client: %w", err)
	}
	firewalls, err := compute.NewFirewallsRESTClient(ctx)
	if err != nil {
		_ = instances.Close()
		return nil, fmt.Errorf("firewalls client: %w", err)
	}
	disks, err := compute.NewDisksRESTClient(ctx)
	if err != nil {
		_ = instances.Close()
		_ = firewalls.Close()
		return nil, fmt.Errorf("disks client: %w", err)
	}
	networks, err := compute.NewNetworksRESTClient(ctx)
	if err != nil {
		_ = instances.Close()
		_ = firewalls.Close()
		_ = disks.Close()
		return nil, fmt.Errorf("networks client: %w", err)
	}
	subnetworks, err := compute.NewSubnetworksRESTClient(ctx)
	if err != nil {
		_ = instances.Close()
		_ = firewalls.Close()
		_ = disks.Close()
		_ = networks.Close()
		return nil, fmt.Errorf("subnetworks client: %w", err)
	}
	machineTypes, err := compute.NewMachineTypesRESTClient(ctx)
	if err != nil {
		_ = instances.Close()
		_ = firewalls.Close()
		_ = disks.Close()
		_ = networks.Close()
		_ = subnetworks.Close()
		return nil, fmt.Errorf("machine types client: %w", err)
	}

	return &Client{
		Instances:    instances,
		Disks:        disks,
		Firewalls:    firewalls,
		Networks:     networks,
		Subnetworks:  subnetworks,
		MachineTypes: machineTypes,
	}, nil
}

func (c *Client) Close() error {
	var err error
	if c.Instances != nil {
		err = c.Instances.Close()
	}
	if c.Disks != nil {
		_ = c.Disks.Close()
	}
	if c.Firewalls != nil {
		_ = c.Firewalls.Close()
	}
	if c.Networks != nil {
		_ = c.Networks.Close()
	}
	if c.Subnetworks != nil {
		_ = c.Subnetworks.Close()
	}
	if c.MachineTypes != nil {
		_ = c.MachineTypes.Close()
	}
	return err
}
