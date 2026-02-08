package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"

	"gpunow/internal/cloudinit"
	"gpunow/internal/config"
	"gpunow/internal/gcp"
	"gpunow/internal/instance"
	"gpunow/internal/ssh"
	"gpunow/internal/ui"
	"gpunow/internal/validate"
)

type Service struct {
	Compute gcp.Compute
	Config  *config.Config
	Builder *instance.Builder
	UI      *ui.UI
	Logger  *zap.Logger
}

type StartOptions struct {
	NumInstances      int
	SSHUser           string
	SSHPublicKey      string
	MachineType       string
	MaxRunHours       int
	TerminationAction string
	DiskSizeGB        int
	KeepDisks         bool
}

type StopOptions struct {
	Delete      bool
	KeepDisks   bool
	DeleteDisks bool
}

type UpdateOptions struct {
	MaxRunHours int
}

func NewService(compute gcp.Compute, cfg *config.Config, uiPrinter *ui.UI, logger *zap.Logger) *Service {
	return &Service{
		Compute: compute,
		Config:  cfg,
		Builder: instance.NewBuilder(cfg),
		UI:      uiPrinter,
		Logger:  logger,
	}
}

func (s *Service) Start(ctx context.Context, clusterName string, opts StartOptions) error {
	if !validate.IsResourceName(clusterName) {
		return fmt.Errorf("invalid cluster name: %s", clusterName)
	}
	if opts.NumInstances <= 0 {
		return fmt.Errorf("num-instances must be >= 1")
	}

	split := s.UI.StartLiveSplit()
	if split != nil {
		defer split.Stop()
	}
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone
	region, err := gcp.RegionFromZone(zone)
	if err != nil {
		return err
	}

	networkName := s.clusterNetworkName(clusterName)
	subnetName := s.clusterSubnetName(networkName)
	networkURL := gcp.GlobalResource(project, "networks", networkName)
	subnetURL := gcp.RegionResource(project, region, "subnetworks", subnetName)

	subnetCIDR, err := DeriveSubnetCIDR(s.Config.Cluster.SubnetCIDRBase, s.Config.Cluster.SubnetPrefix, clusterName)
	if err != nil {
		return err
	}

	internalRule := fmt.Sprintf("%s-internal", networkName)
	sshRule := fmt.Sprintf("%s-ssh", networkName)
	portsRule := fmt.Sprintf("%s-ports", networkName)

	cloudInit, err := cloudinit.Render(s.Config.Paths.CloudInitFile, s.Config.Paths.SetupScript, s.Config.Paths.ZshrcFile)
	if err != nil {
		return err
	}

	instanceNames := make([]string, 0, opts.NumInstances)
	for i := 0; i < opts.NumInstances; i++ {
		instanceNames = append(instanceNames, s.instanceName(clusterName, i))
	}
	resourceTasks := []string{
		fmt.Sprintf("network %s", networkName),
		fmt.Sprintf("subnetwork %s", subnetName),
		fmt.Sprintf("firewall %s", internalRule),
		fmt.Sprintf("firewall %s", sshRule),
		fmt.Sprintf("firewall %s", portsRule),
	}
	taskNames := append([]string{}, resourceTasks...)
	for _, name := range instanceNames {
		taskNames = append(taskNames, fmt.Sprintf("instance %s", name))
	}
	progress := s.UI.TaskList("Creating", taskNames)
	resourceTaskCount := len(resourceTasks)

	if err := s.ensureNetwork(ctx, project, networkName); err != nil {
		progress.Stop()
		return err
	}
	progress.MarkDone(0, fmt.Sprintf("Ready networks/%s", networkName))

	if err := s.ensureSubnetwork(ctx, project, region, subnetName, subnetCIDR, networkURL); err != nil {
		progress.Stop()
		return err
	}
	progress.MarkDone(1, fmt.Sprintf("Ready subnetworks/%s", subnetName))

	network := gcp.GlobalResource(project, "networks", networkName)
	clusterTag := s.clusterTag(clusterName)
	internalFirewall := &computepb.Firewall{
		Name:         proto.String(internalRule),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		TargetTags:   []string{clusterTag},
		SourceRanges: []string{subnetCIDR},
		Allowed: []*computepb.Allowed{
			{IPProtocol: proto.String("tcp"), Ports: []string{"0-65535"}},
			{IPProtocol: proto.String("udp"), Ports: []string{"0-65535"}},
			{IPProtocol: proto.String("icmp")},
		},
	}
	sshFirewall := &computepb.Firewall{
		Name:         proto.String(sshRule),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		TargetTags:   []string{clusterTag},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: proto.String("tcp"),
			Ports:      []string{"22"},
		}},
	}
	portsFirewall := &computepb.Firewall{
		Name:         proto.String(portsRule),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		TargetTags:   []string{clusterTag},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: proto.String("tcp"),
			Ports:      portsToStrings(s.Config.Network.Ports),
		}},
	}
	if err := s.ensureFirewall(ctx, project, internalRule, internalFirewall); err != nil {
		progress.Stop()
		return err
	}
	progress.MarkDone(2, fmt.Sprintf("Ready firewalls/%s", internalRule))
	if err := s.ensureFirewall(ctx, project, sshRule, sshFirewall); err != nil {
		progress.Stop()
		return err
	}
	progress.MarkDone(3, fmt.Sprintf("Ready firewalls/%s", sshRule))
	if err := s.ensureFirewall(ctx, project, portsRule, portsFirewall); err != nil {
		progress.Stop()
		return err
	}
	progress.MarkDone(4, fmt.Sprintf("Ready firewalls/%s", portsRule))

	group, groupCtx := errgroup.WithContext(ctx)
	for i := 0; i < opts.NumInstances; i++ {
		name := s.instanceName(clusterName, i)
		clusterIndex := i
		progressIndex := resourceTaskCount + i
		role := "worker"
		publicIP := true
		if i == 0 {
			role = "master"
		}
		group.Go(func() error {
			labels := map[string]string{
				"cluster":       clusterName,
				"cluster_index": strconv.Itoa(clusterIndex),
				"cluster_role":  role,
			}
			metadata := map[string]string{
				"cluster":       clusterName,
				"cluster_index": strconv.Itoa(clusterIndex),
				"cluster_role":  role,
			}
			if opts.SSHUser != "" && opts.SSHPublicKey != "" {
				metadata["ssh-keys"] = fmt.Sprintf("%s:%s", opts.SSHUser, opts.SSHPublicKey)
			}
			tags := s.clusterTags(clusterName, role == "master")

			instanceObj, err := s.getInstance(groupCtx, name)
			if err != nil {
				return err
			}
			if instanceObj != nil {
				if err := s.ensureInstanceTags(groupCtx, instanceObj, tags); err != nil {
					return err
				}
				if opts.SSHUser != "" && opts.SSHPublicKey != "" {
					if err := s.ensureInstanceSSHKey(groupCtx, name, opts.SSHUser, opts.SSHPublicKey); err != nil {
						return err
					}
				}
				if instanceObj.GetStatus() == "RUNNING" {
					progress.MarkDone(progressIndex, fmt.Sprintf("Already running %s", name))
					return nil
				}
				call := s.api("compute.instances.start", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Starting %s", name))
				op, err := s.Compute.StartInstance(groupCtx, &computepb.StartInstanceRequest{
					Project:  project,
					Zone:     zone,
					Instance: name,
				})
				if err != nil {
					call.Stop()
					return err
				}
				if err := s.waitWithProgress(groupCtx, call, op, func(p int32) { progress.Update(progressIndex, p) }); err != nil {
					return err
				}
				progress.MarkDone(progressIndex, fmt.Sprintf("Started %s", name))
				return nil
			}

			instanceReq, err := s.Builder.Build(groupCtx, s.Compute, instance.Options{
				Name:              name,
				Network:           networkURL,
				Subnetwork:        subnetURL,
				PublicIP:          publicIP,
				Tags:              tags,
				CloudInit:         cloudInit,
				Labels:            labels,
				Metadata:          metadata,
				MachineType:       strings.TrimSpace(opts.MachineType),
				MaxRunHours:       opts.MaxRunHours,
				TerminationAction: strings.ToUpper(strings.TrimSpace(opts.TerminationAction)),
				DiskSizeGB:        opts.DiskSizeGB,
				DiskAutoDelete:    diskAutoDeleteOverride(opts.KeepDisks),
			})
			if err != nil {
				return err
			}
			call := s.api("compute.instances.insert", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Creating %s", name))
			op, err := s.Compute.InsertInstance(groupCtx, instanceReq)
			if err != nil {
				call.Stop()
				return err
			}
			if err := s.waitWithProgress(groupCtx, call, op, func(p int32) { progress.Update(progressIndex, p) }); err != nil {
				return err
			}
			if opts.SSHUser != "" && opts.SSHPublicKey != "" {
				if err := s.ensureInstanceSSHKey(groupCtx, name, opts.SSHUser, opts.SSHPublicKey); err != nil {
					return err
				}
			}
			progress.MarkDone(progressIndex, fmt.Sprintf("Created %s", name))
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		progress.Stop()
		return err
	}
	progress.Stop()
	return nil
}

func (s *Service) Stop(ctx context.Context, clusterName string, opts StopOptions) error {
	if !validate.IsResourceName(clusterName) {
		return fmt.Errorf("invalid cluster name: %s", clusterName)
	}
	if !opts.Delete && (opts.KeepDisks || opts.DeleteDisks) {
		return fmt.Errorf("--delete-disks requires --delete")
	}

	split := s.UI.StartLiveSplit()
	if split != nil {
		defer split.Stop()
	}
	instances, err := s.listClusterInstances(ctx, clusterName)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		if split != nil {
			split.Stop()
		}
		s.UI.Infof("No instances found for cluster %s", clusterName)
		return nil
	}

	project := s.Config.Project.ID
	zone := s.Config.Project.Zone
	region, err := gcp.RegionFromZone(zone)
	if err != nil {
		return err
	}

	label := "Stopping instance"
	networkName := s.clusterNetworkName(clusterName)
	subnetName := s.clusterSubnetName(networkName)
	extraTasks := []string{}
	if opts.Delete {
		label = "Deleting"
		extraTasks = append(extraTasks,
			fmt.Sprintf("firewalls/%s-internal", networkName),
			fmt.Sprintf("firewalls/%s-ssh", networkName),
			fmt.Sprintf("firewalls/%s-ports", networkName),
			fmt.Sprintf("subnetworks/%s", subnetName),
			fmt.Sprintf("networks/%s", networkName),
		)
	}
	taskNames := instanceNames(instances)
	if opts.Delete {
		for i, name := range taskNames {
			taskNames[i] = fmt.Sprintf("instance %s", name)
		}
	}
	taskNames = append(taskNames, extraTasks...)
	progress := s.UI.TaskList(label, taskNames)

	group, groupCtx := errgroup.WithContext(ctx)
	for idx, inst := range instances {
		index := idx
		name := inst.GetName()
		group.Go(func() error {
			if opts.Delete {
				autoDelete := !opts.KeepDisks
				if opts.DeleteDisks {
					autoDelete = true
				}
				if err := s.setAutoDelete(groupCtx, name, inst, autoDelete); err != nil {
					return err
				}
				call := s.api("compute.instances.delete", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Deleting %s", name))
				op, err := s.Compute.DeleteInstance(groupCtx, &computepb.DeleteInstanceRequest{
					Project:  project,
					Zone:     zone,
					Instance: name,
				})
				if err != nil {
					call.Stop()
					return err
				}
				if err := s.waitWithProgress(groupCtx, call, op, func(p int32) { progress.Update(index, p) }); err != nil {
					return err
				}
				progress.MarkDone(index, fmt.Sprintf("Deleted %s", name))
				return nil
			}

			if inst.GetStatus() == "TERMINATED" {
				progress.MarkDone(index, fmt.Sprintf("Already terminated %s", name))
				return nil
			}
			call := s.api("compute.instances.stop", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Stopping %s", name))
			op, err := s.Compute.StopInstance(groupCtx, &computepb.StopInstanceRequest{
				Project:  project,
				Zone:     zone,
				Instance: name,
			})
			if err != nil {
				call.Stop()
				return err
			}
			if err := s.waitWithProgress(groupCtx, call, op, func(p int32) { progress.Update(index, p) }); err != nil {
				return err
			}
			progress.MarkDone(index, fmt.Sprintf("Stopped %s", name))
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		progress.Stop()
		return err
	}

	if opts.Delete {
		base := len(instances)
		firewalls := []string{
			fmt.Sprintf("%s-internal", networkName),
			fmt.Sprintf("%s-ssh", networkName),
			fmt.Sprintf("%s-ports", networkName),
		}
		for i, rule := range firewalls {
			idx := base + i
			call := s.api("compute.firewalls.delete", gcp.GlobalResource(project, "firewalls", rule), fmt.Sprintf("Deleting firewall %s", rule))
			op, err := s.Compute.DeleteFirewall(ctx, &computepb.DeleteFirewallRequest{
				Project:  project,
				Firewall: rule,
			})
			if err != nil {
				call.Stop()
				if gcp.IsNotFound(err) {
					progress.MarkDone(idx, fmt.Sprintf("Deleted firewalls/%s", rule))
					continue
				}
				return err
			}
			if err := s.waitWithProgress(ctx, call, op, func(p int32) { progress.Update(idx, p) }); err != nil {
				return err
			}
			progress.MarkDone(idx, fmt.Sprintf("Deleted firewalls/%s", rule))
		}

		subnetIdx := base + len(firewalls)
		call := s.api("compute.subnetworks.delete", gcp.RegionResource(project, region, "subnetworks", subnetName), fmt.Sprintf("Deleting subnetwork %s", subnetName))
		op, err := s.Compute.DeleteSubnetwork(ctx, &computepb.DeleteSubnetworkRequest{
			Project:    project,
			Region:     region,
			Subnetwork: subnetName,
		})
		if err != nil {
			call.Stop()
			if !gcp.IsNotFound(err) {
				return err
			}
			progress.MarkDone(subnetIdx, fmt.Sprintf("Deleted subnetworks/%s", subnetName))
		} else {
			if err := s.waitWithProgress(ctx, call, op, func(p int32) { progress.Update(subnetIdx, p) }); err != nil {
				return err
			}
			progress.MarkDone(subnetIdx, fmt.Sprintf("Deleted subnetworks/%s", subnetName))
		}

		networkIdx := subnetIdx + 1
		call = s.api("compute.networks.delete", gcp.GlobalResource(project, "networks", networkName), fmt.Sprintf("Deleting network %s", networkName))
		op, err = s.Compute.DeleteNetwork(ctx, &computepb.DeleteNetworkRequest{
			Project: project,
			Network: networkName,
		})
		if err != nil {
			call.Stop()
			if !gcp.IsNotFound(err) {
				return err
			}
			progress.MarkDone(networkIdx, fmt.Sprintf("Deleted networks/%s", networkName))
		} else {
			if err := s.waitWithProgress(ctx, call, op, func(p int32) { progress.Update(networkIdx, p) }); err != nil {
				return err
			}
			progress.MarkDone(networkIdx, fmt.Sprintf("Deleted networks/%s", networkName))
		}
	}

	progress.Stop()
	return nil
}

func (s *Service) Show(ctx context.Context, clusterName string) error {
	if !validate.IsResourceName(clusterName) {
		return fmt.Errorf("invalid cluster name: %s", clusterName)
	}

	instances, err := s.listClusterInstances(ctx, clusterName)
	if err != nil {
		return err
	}

	s.UI.Heading("Cluster")
	s.UI.Infof("Name: %s", clusterName)
	s.UI.Infof("Instances: %d", len(instances))

	for _, inst := range instances {
		external := ""
		internal := ""
		if len(inst.GetNetworkInterfaces()) > 0 {
			iface := inst.GetNetworkInterfaces()[0]
			internal = iface.GetNetworkIP()
			if len(iface.GetAccessConfigs()) > 0 {
				external = iface.GetAccessConfigs()[0].GetNatIP()
			}
		}
		line := fmt.Sprintf("%s (%s)", inst.GetName(), inst.GetStatus())
		if external != "" {
			line = fmt.Sprintf("%s %s", line, external)
		}
		if internal != "" {
			line = fmt.Sprintf("%s [%s]", line, internal)
		}
		s.UI.Infof("%s", line)
	}

	return nil
}

func (s *Service) Update(ctx context.Context, clusterName string, opts UpdateOptions) error {
	if !validate.IsResourceName(clusterName) {
		return fmt.Errorf("invalid cluster name: %s", clusterName)
	}
	if opts.MaxRunHours <= 0 {
		return fmt.Errorf("max-hours must be >= 1")
	}

	split := s.UI.StartLiveSplit()
	if split != nil {
		defer split.Stop()
	}
	instances, err := s.listClusterInstances(ctx, clusterName)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		if split != nil {
			split.Stop()
		}
		s.UI.Infof("No instances found for cluster %s", clusterName)
		return nil
	}

	project := s.Config.Project.ID
	zone := s.Config.Project.Zone
	scheduling := s.Builder.Scheduling(opts.MaxRunHours)
	label := "Updating instance"
	progress := s.UI.TaskList(label, instanceNames(instances))
	group, groupCtx := errgroup.WithContext(ctx)
	for idx, inst := range instances {
		index := idx
		name := inst.GetName()
		group.Go(func() error {
			if inst.GetStatus() != "TERMINATED" {
				progress.MarkWarning(index, fmt.Sprintf("%s must be TERMINATED to update max run duration", name))
				return nil
			}
			call := s.api("compute.instances.setScheduling", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Updating scheduling for %s", name))
			op, err := s.Compute.SetInstanceScheduling(groupCtx, &computepb.SetSchedulingInstanceRequest{
				Project:            project,
				Zone:               zone,
				Instance:           name,
				SchedulingResource: scheduling,
			})
			if err != nil {
				call.Stop()
				return err
			}
			if err := s.waitWithProgress(groupCtx, call, op, func(p int32) { progress.Update(index, p) }); err != nil {
				return err
			}
			progress.MarkDone(index, fmt.Sprintf("Updated %s", name))
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		progress.Stop()
		return err
	}
	progress.Stop()
	return nil
}

func (s *Service) listClusterInstances(ctx context.Context, clusterName string) ([]*computepb.Instance, error) {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	filter := fmt.Sprintf("labels.cluster = %q", clusterName)
	call := s.api("compute.instances.list", fmt.Sprintf("projects/%s/zones/%s/instances?filter=%s", project, zone, filter), "")
	it := s.Compute.ListInstances(ctx, &computepb.ListInstancesRequest{
		Project: project,
		Zone:    zone,
		Filter:  proto.String(filter),
	})

	var instances []*computepb.Instance
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			call.Stop()
			return nil, err
		}
		instances = append(instances, inst)
	}
	call.Stop()
	return instances, nil
}

func (s *Service) getInstance(ctx context.Context, name string) (*computepb.Instance, error) {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	call := s.api("compute.instances.get", gcp.ZoneResource(project, zone, "instances", name), "")
	instanceObj, err := s.Compute.GetInstance(ctx, &computepb.GetInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
	})
	call.Stop()
	if err != nil {
		if gcp.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return instanceObj, nil
}

func (s *Service) ensureNetwork(ctx context.Context, project, networkName string) error {
	getCall := s.api("compute.networks.get", gcp.GlobalResource(project, "networks", networkName), "")
	_, err := s.Compute.GetNetwork(ctx, &computepb.GetNetworkRequest{
		Project: project,
		Network: networkName,
	})
	getCall.Stop()
	if err == nil {
		return nil
	}
	if !gcp.IsNotFound(err) {
		return err
	}

	call := s.api("compute.networks.insert", gcp.GlobalResource(project, "networks", networkName), fmt.Sprintf("Creating network %s", networkName))
	op, err := s.Compute.InsertNetwork(ctx, &computepb.InsertNetworkRequest{
		Project: project,
		NetworkResource: &computepb.Network{
			Name:                  proto.String(networkName),
			AutoCreateSubnetworks: proto.Bool(false),
		},
	})
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) ensureSubnetwork(ctx context.Context, project, region, subnetName, cidr, networkURL string) error {
	getCall := s.api("compute.subnetworks.get", gcp.RegionResource(project, region, "subnetworks", subnetName), "")
	_, err := s.Compute.GetSubnetwork(ctx, &computepb.GetSubnetworkRequest{
		Project:    project,
		Region:     region,
		Subnetwork: subnetName,
	})
	getCall.Stop()
	if err == nil {
		return nil
	}
	if !gcp.IsNotFound(err) {
		return err
	}

	call := s.api("compute.subnetworks.insert", gcp.RegionResource(project, region, "subnetworks", subnetName), fmt.Sprintf("Creating subnetwork %s", subnetName))
	op, err := s.Compute.InsertSubnetwork(ctx, &computepb.InsertSubnetworkRequest{
		Project: project,
		Region:  region,
		SubnetworkResource: &computepb.Subnetwork{
			Name:        proto.String(subnetName),
			IpCidrRange: proto.String(cidr),
			Network:     proto.String(networkURL),
		},
	})
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) ensureFirewalls(ctx context.Context, project, networkName, cidr, clusterName string) error {
	internalRule := fmt.Sprintf("%s-internal", networkName)
	sshRule := fmt.Sprintf("%s-ssh", networkName)
	portsRule := fmt.Sprintf("%s-ports", networkName)

	network := gcp.GlobalResource(project, "networks", networkName)
	clusterTag := s.clusterTag(clusterName)

	internalFirewall := &computepb.Firewall{
		Name:         proto.String(internalRule),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		TargetTags:   []string{clusterTag},
		SourceRanges: []string{cidr},
		Allowed: []*computepb.Allowed{
			{IPProtocol: proto.String("tcp"), Ports: []string{"0-65535"}},
			{IPProtocol: proto.String("udp"), Ports: []string{"0-65535"}},
			{IPProtocol: proto.String("icmp")},
		},
	}

	sshFirewall := &computepb.Firewall{
		Name:         proto.String(sshRule),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		TargetTags:   []string{clusterTag},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: proto.String("tcp"),
			Ports:      []string{"22"},
		}},
	}

	portsFirewall := &computepb.Firewall{
		Name:         proto.String(portsRule),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		TargetTags:   []string{clusterTag},
		SourceRanges: []string{"0.0.0.0/0"},
		Allowed: []*computepb.Allowed{{
			IPProtocol: proto.String("tcp"),
			Ports:      portsToStrings(s.Config.Network.Ports),
		}},
	}

	if err := s.ensureFirewall(ctx, project, internalRule, internalFirewall); err != nil {
		return err
	}
	if err := s.ensureFirewall(ctx, project, sshRule, sshFirewall); err != nil {
		return err
	}
	if err := s.ensureFirewall(ctx, project, portsRule, portsFirewall); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensureFirewall(ctx context.Context, project, name string, rule *computepb.Firewall) error {
	getCall := s.api("compute.firewalls.get", gcp.GlobalResource(project, "firewalls", name), "")
	_, err := s.Compute.GetFirewall(ctx, &computepb.GetFirewallRequest{
		Project:  project,
		Firewall: name,
	})
	getCall.Stop()
	if err != nil {
		if gcp.IsNotFound(err) {
			call := s.api("compute.firewalls.insert", gcp.GlobalResource(project, "firewalls", name), fmt.Sprintf("Updating firewall %s", name))
			op, err := s.Compute.InsertFirewall(ctx, &computepb.InsertFirewallRequest{
				Project:          project,
				FirewallResource: rule,
			})
			if err != nil {
				call.Stop()
				return err
			}
			return s.wait(ctx, call, op)
		}
		return err
	}

	call := s.api("compute.firewalls.patch", gcp.GlobalResource(project, "firewalls", name), fmt.Sprintf("Updating firewall %s", name))
	op, err := s.Compute.PatchFirewall(ctx, &computepb.PatchFirewallRequest{
		Project:          project,
		Firewall:         name,
		FirewallResource: rule,
	})
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) deleteFirewalls(ctx context.Context, project, networkName string) error {
	rules := []string{
		fmt.Sprintf("%s-internal", networkName),
		fmt.Sprintf("%s-ssh", networkName),
		fmt.Sprintf("%s-ports", networkName),
	}

	for _, rule := range rules {
		call := s.api("compute.firewalls.delete", gcp.GlobalResource(project, "firewalls", rule), fmt.Sprintf("Deleting firewall %s", rule))
		op, err := s.Compute.DeleteFirewall(ctx, &computepb.DeleteFirewallRequest{
			Project:  project,
			Firewall: rule,
		})
		if err != nil {
			call.Stop()
			if gcp.IsNotFound(err) {
				continue
			}
			return err
		}
		if err := s.wait(ctx, call, op); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) deleteSubnetwork(ctx context.Context, project, region, subnetName string) error {
	call := s.api("compute.subnetworks.delete", gcp.RegionResource(project, region, "subnetworks", subnetName), fmt.Sprintf("Deleting subnetwork %s", subnetName))
	op, err := s.Compute.DeleteSubnetwork(ctx, &computepb.DeleteSubnetworkRequest{
		Project:    project,
		Region:     region,
		Subnetwork: subnetName,
	})
	if err != nil {
		call.Stop()
		if gcp.IsNotFound(err) {
			return nil
		}
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) deleteNetwork(ctx context.Context, project, networkName string) error {
	call := s.api("compute.networks.delete", gcp.GlobalResource(project, "networks", networkName), fmt.Sprintf("Deleting network %s", networkName))
	op, err := s.Compute.DeleteNetwork(ctx, &computepb.DeleteNetworkRequest{
		Project: project,
		Network: networkName,
	})
	if err != nil {
		call.Stop()
		if gcp.IsNotFound(err) {
			return nil
		}
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) setAutoDelete(ctx context.Context, name string, instanceObj *computepb.Instance, autoDelete bool) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	for _, disk := range instanceObj.GetDisks() {
		req := &computepb.SetDiskAutoDeleteInstanceRequest{
			Project:    project,
			Zone:       zone,
			Instance:   name,
			DeviceName: disk.GetDeviceName(),
			AutoDelete: autoDelete,
		}
		call := s.api("compute.instances.setDiskAutoDelete", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Updating disk %s", disk.GetDeviceName()))
		op, err := s.Compute.SetDiskAutoDelete(ctx, req)
		if err != nil {
			call.Stop()
			return err
		}
		if err := s.wait(ctx, call, op); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureInstanceTags(ctx context.Context, instanceObj *computepb.Instance, tags []string) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	existing := instanceObj.GetTags().GetItems()
	if containsAll(existing, tags) {
		return nil
	}

	req := &computepb.SetTagsInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instanceObj.GetName(),
		TagsResource: &computepb.Tags{
			Items:       tags,
			Fingerprint: proto.String(instanceObj.GetTags().GetFingerprint()),
		},
	}
	call := s.api("compute.instances.setTags", gcp.ZoneResource(project, zone, "instances", instanceObj.GetName()), fmt.Sprintf("Updating tags for %s", instanceObj.GetName()))
	op, err := s.Compute.SetInstanceTags(ctx, req)
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) api(action, resource, label string) *ui.APICall {
	return s.UI.APICall(action, resource, label)
}

func (s *Service) wait(ctx context.Context, call *ui.APICall, op *compute.Operation) error {
	err := op.Wait(ctx)
	if call != nil {
		call.Stop()
	}
	return err
}

func (s *Service) waitWithProgress(ctx context.Context, call *ui.APICall, op *compute.Operation, update func(int32)) error {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := op.Poll(ctx); err != nil {
			if call != nil {
				call.Stop()
			}
			return err
		}
		if update != nil {
			update(op.Proto().GetProgress())
		}
		if op.Done() {
			if call != nil {
				call.Stop()
			}
			return nil
		}
		select {
		case <-ctx.Done():
			if call != nil {
				call.Stop()
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) ensureInstanceSSHKey(ctx context.Context, name, user, publicKey string) error {
	return ssh.EnsureInstanceSSHKey(ctx, s.Compute, s.Config, name, user, publicKey)
}

func (s *Service) clusterNetworkName(clusterName string) string {
	return fmt.Sprintf("%s-%s", s.Config.Cluster.NetworkNamePrefix, clusterName)
}

func (s *Service) clusterSubnetName(networkName string) string {
	return fmt.Sprintf("%s-subnet", networkName)
}

func (s *Service) clusterTag(clusterName string) string {
	return fmt.Sprintf("cluster-%s", clusterName)
}

func (s *Service) masterTag(clusterName string) string {
	return fmt.Sprintf("cluster-%s-master", clusterName)
}

func (s *Service) clusterTags(clusterName string, master bool) []string {
	tags := []string{s.clusterTag(clusterName)}
	if master {
		tags = append(tags, s.masterTag(clusterName))
		tags = append(tags, s.Config.Network.TagsBase...)
	}
	return tags
}

func (s *Service) instanceName(clusterName string, index int) string {
	return fmt.Sprintf("%s-%d", clusterName, index)
}

func diskAutoDeleteOverride(keepDisks bool) *bool {
	if !keepDisks {
		return nil
	}
	value := false
	return &value
}

func portsToStrings(ports []int) []string {
	out := make([]string, 0, len(ports))
	for _, port := range ports {
		out = append(out, strconv.Itoa(port))
	}
	return out
}

func instanceNames(instances []*computepb.Instance) []string {
	names := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		names = append(names, inst.GetName())
	}
	return names
}

func containsAll(existing []string, required []string) bool {
	seen := map[string]struct{}{}
	for _, item := range existing {
		seen[item] = struct{}{}
	}
	for _, item := range required {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}
