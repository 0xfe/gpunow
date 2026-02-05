package gcp

import (
	"context"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
)

type Compute interface {
	GetInstance(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error)
	InsertInstance(ctx context.Context, req *computepb.InsertInstanceRequest) (*compute.Operation, error)
	StartInstance(ctx context.Context, req *computepb.StartInstanceRequest) (*compute.Operation, error)
	StopInstance(ctx context.Context, req *computepb.StopInstanceRequest) (*compute.Operation, error)
	DeleteInstance(ctx context.Context, req *computepb.DeleteInstanceRequest) (*compute.Operation, error)
	SetInstanceScheduling(ctx context.Context, req *computepb.SetSchedulingInstanceRequest) (*compute.Operation, error)
	SetInstanceTags(ctx context.Context, req *computepb.SetTagsInstanceRequest) (*compute.Operation, error)
	SetDiskAutoDelete(ctx context.Context, req *computepb.SetDiskAutoDeleteInstanceRequest) (*compute.Operation, error)
	ListInstances(ctx context.Context, req *computepb.ListInstancesRequest) *compute.InstanceIterator

	GetMachineType(ctx context.Context, req *computepb.GetMachineTypeRequest) (*computepb.MachineType, error)

	GetDisk(ctx context.Context, req *computepb.GetDiskRequest) (*computepb.Disk, error)

	GetFirewall(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error)
	InsertFirewall(ctx context.Context, req *computepb.InsertFirewallRequest) (*compute.Operation, error)
	PatchFirewall(ctx context.Context, req *computepb.PatchFirewallRequest) (*compute.Operation, error)
	DeleteFirewall(ctx context.Context, req *computepb.DeleteFirewallRequest) (*compute.Operation, error)

	GetNetwork(ctx context.Context, req *computepb.GetNetworkRequest) (*computepb.Network, error)
	InsertNetwork(ctx context.Context, req *computepb.InsertNetworkRequest) (*compute.Operation, error)
	DeleteNetwork(ctx context.Context, req *computepb.DeleteNetworkRequest) (*compute.Operation, error)

	GetSubnetwork(ctx context.Context, req *computepb.GetSubnetworkRequest) (*computepb.Subnetwork, error)
	InsertSubnetwork(ctx context.Context, req *computepb.InsertSubnetworkRequest) (*compute.Operation, error)
	DeleteSubnetwork(ctx context.Context, req *computepb.DeleteSubnetworkRequest) (*compute.Operation, error)
}

func (c *Client) GetInstance(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error) {
	return c.Instances.Get(ctx, req)
}

func (c *Client) InsertInstance(ctx context.Context, req *computepb.InsertInstanceRequest) (*compute.Operation, error) {
	return c.Instances.Insert(ctx, req)
}

func (c *Client) StartInstance(ctx context.Context, req *computepb.StartInstanceRequest) (*compute.Operation, error) {
	return c.Instances.Start(ctx, req)
}

func (c *Client) StopInstance(ctx context.Context, req *computepb.StopInstanceRequest) (*compute.Operation, error) {
	return c.Instances.Stop(ctx, req)
}

func (c *Client) DeleteInstance(ctx context.Context, req *computepb.DeleteInstanceRequest) (*compute.Operation, error) {
	return c.Instances.Delete(ctx, req)
}

func (c *Client) SetInstanceScheduling(ctx context.Context, req *computepb.SetSchedulingInstanceRequest) (*compute.Operation, error) {
	return c.Instances.SetScheduling(ctx, req)
}

func (c *Client) SetInstanceTags(ctx context.Context, req *computepb.SetTagsInstanceRequest) (*compute.Operation, error) {
	return c.Instances.SetTags(ctx, req)
}

func (c *Client) SetDiskAutoDelete(ctx context.Context, req *computepb.SetDiskAutoDeleteInstanceRequest) (*compute.Operation, error) {
	return c.Instances.SetDiskAutoDelete(ctx, req)
}

func (c *Client) ListInstances(ctx context.Context, req *computepb.ListInstancesRequest) *compute.InstanceIterator {
	return c.Instances.List(ctx, req)
}

func (c *Client) GetMachineType(ctx context.Context, req *computepb.GetMachineTypeRequest) (*computepb.MachineType, error) {
	return c.MachineTypes.Get(ctx, req)
}

func (c *Client) GetDisk(ctx context.Context, req *computepb.GetDiskRequest) (*computepb.Disk, error) {
	return c.Disks.Get(ctx, req)
}

func (c *Client) GetFirewall(ctx context.Context, req *computepb.GetFirewallRequest) (*computepb.Firewall, error) {
	return c.Firewalls.Get(ctx, req)
}

func (c *Client) InsertFirewall(ctx context.Context, req *computepb.InsertFirewallRequest) (*compute.Operation, error) {
	return c.Firewalls.Insert(ctx, req)
}

func (c *Client) PatchFirewall(ctx context.Context, req *computepb.PatchFirewallRequest) (*compute.Operation, error) {
	return c.Firewalls.Patch(ctx, req)
}

func (c *Client) DeleteFirewall(ctx context.Context, req *computepb.DeleteFirewallRequest) (*compute.Operation, error) {
	return c.Firewalls.Delete(ctx, req)
}

func (c *Client) GetNetwork(ctx context.Context, req *computepb.GetNetworkRequest) (*computepb.Network, error) {
	return c.Networks.Get(ctx, req)
}

func (c *Client) InsertNetwork(ctx context.Context, req *computepb.InsertNetworkRequest) (*compute.Operation, error) {
	return c.Networks.Insert(ctx, req)
}

func (c *Client) DeleteNetwork(ctx context.Context, req *computepb.DeleteNetworkRequest) (*compute.Operation, error) {
	return c.Networks.Delete(ctx, req)
}

func (c *Client) GetSubnetwork(ctx context.Context, req *computepb.GetSubnetworkRequest) (*computepb.Subnetwork, error) {
	return c.Subnetworks.Get(ctx, req)
}

func (c *Client) InsertSubnetwork(ctx context.Context, req *computepb.InsertSubnetworkRequest) (*compute.Operation, error) {
	return c.Subnetworks.Insert(ctx, req)
}

func (c *Client) DeleteSubnetwork(ctx context.Context, req *computepb.DeleteSubnetworkRequest) (*compute.Operation, error) {
	return c.Subnetworks.Delete(ctx, req)
}
