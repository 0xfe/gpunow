package vm

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"gpunow/internal/cloudinit"
	"gpunow/internal/config"
	"gpunow/internal/gcp"
	"gpunow/internal/instance"
	"gpunow/internal/ui"
)

type Service struct {
	Compute gcp.Compute
	Config  *config.Config
	Builder *instance.Builder
	UI      *ui.UI
	Logger  *zap.Logger
}

type StartOptions struct {
	Ports         []int
	MaxRunHours   int
	MaxHoursSet   bool
	AllowCreate   bool
	SkipFirewall  bool
	SkipTagUpdate bool
}

type StopOptions struct {
	Delete    bool
	KeepDisks bool
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

func (s *Service) Start(ctx context.Context, name string, opts StartOptions) error {
	ports := opts.Ports
	if len(ports) == 0 {
		ports = s.Config.Network.Ports
	}

	if !opts.SkipFirewall {
		if err := s.ensureFirewall(ctx, name, ports); err != nil {
			return err
		}
	}

	instanceObj, err := s.getInstance(ctx, name)
	if err != nil {
		return err
	}

	if instanceObj != nil {
		if !opts.SkipTagUpdate {
			if err := s.ensureTags(ctx, instanceObj, name); err != nil {
				return err
			}
		}
		status := instanceObj.GetStatus()
		switch status {
		case "RUNNING":
			s.UI.Infof("%s is already RUNNING", name)
			if opts.MaxHoursSet {
				s.UI.Warnf("Max run duration can only be updated while stopped (TERMINATED)")
			}
			return nil
		case "TERMINATED":
			if opts.MaxHoursSet {
				if err := s.setMaxRunDuration(ctx, name, opts.MaxRunHours); err != nil {
					return err
				}
			}
			return s.startInstance(ctx, name)
		default:
			s.UI.Infof("%s is in state %s; starting anyway", name, status)
			return s.startInstance(ctx, name)
		}
	}

	if !opts.AllowCreate {
		return fmt.Errorf("%s not found; use cluster start to create it", name)
	}

	cloudInit, err := cloudinit.Render(s.Config.Paths.CloudInitFile, s.Config.Paths.SetupScript, s.Config.Paths.ZshrcFile)
	if err != nil {
		return err
	}

	network := gcp.GlobalResource(s.Config.Project.ID, "networks", s.Config.Network.DefaultNetwork)
	tags := append([]string{}, s.Config.Network.TagsBase...)
	tags = append(tags, fmt.Sprintf("%s-ports", name))

	instanceReq, err := s.Builder.Build(ctx, s.Compute, instance.Options{
		Name:        name,
		Network:     network,
		PublicIP:    true,
		Tags:        tags,
		CloudInit:   cloudInit,
		MaxRunHours: opts.MaxRunHours,
	})
	if err != nil {
		return err
	}

	call := s.api("compute.instances.insert", gcp.ZoneResource(s.Config.Project.ID, s.Config.Project.Zone, "instances", name), fmt.Sprintf("Creating %s", name))
	op, err := s.Compute.InsertInstance(ctx, instanceReq)
	if err != nil {
		call.Stop()
		return err
	}
	if err := s.wait(ctx, call, op); err != nil {
		return err
	}

	s.UI.Successf("Created %s", name)
	return nil
}

func (s *Service) Stop(ctx context.Context, name string, opts StopOptions) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	instance, err := s.getInstance(ctx, name)
	if err != nil {
		return err
	}
	if instance == nil {
		s.UI.Infof("%s not found", name)
		return nil
	}

	status := instance.GetStatus()
	if opts.Delete {
		if err := s.setAutoDelete(ctx, name, instance, !opts.KeepDisks); err != nil {
			return err
		}
		call := s.api("compute.instances.delete", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Deleting %s", name))
		op, err := s.Compute.DeleteInstance(ctx, &computepb.DeleteInstanceRequest{
			Project:  project,
			Zone:     zone,
			Instance: name,
		})
		if err != nil {
			call.Stop()
			return err
		}
		if err := s.wait(ctx, call, op); err != nil {
			return err
		}
		s.UI.Successf("Deleted %s", name)
		return nil
	}

	if status == "TERMINATED" {
		s.UI.Infof("%s is already TERMINATED", name)
		return nil
	}

	call := s.api("compute.instances.stop", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Stopping %s", name))
	op, err := s.Compute.StopInstance(ctx, &computepb.StopInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
	})
	if err != nil {
		call.Stop()
		return err
	}
	if err := s.wait(ctx, call, op); err != nil {
		return err
	}
	s.UI.Successf("Stopped %s", name)
	return nil
}

func (s *Service) Update(ctx context.Context, name string, opts UpdateOptions) error {
	instance, err := s.getInstance(ctx, name)
	if err != nil {
		return err
	}
	if instance == nil {
		s.UI.Infof("%s not found", name)
		return nil
	}
	if instance.GetStatus() != "TERMINATED" {
		return fmt.Errorf("instance must be stopped (TERMINATED) before updating max run duration")
	}
	return s.setMaxRunDuration(ctx, name, opts.MaxRunHours)
}

func (s *Service) Show(ctx context.Context, name string) error {
	instance, err := s.getInstance(ctx, name)
	if err != nil {
		return err
	}
	if instance == nil {
		s.UI.Infof("%s not found", name)
		return nil
	}

	externalIP := ""
	if len(instance.GetNetworkInterfaces()) > 0 {
		iface := instance.GetNetworkInterfaces()[0]
		if len(iface.GetAccessConfigs()) > 0 {
			externalIP = iface.GetAccessConfigs()[0].GetNatIP()
		}
	}

	nameStyled := s.UI.Bold(instance.GetName())
	profile := s.Config.Profile
	if profile == "" {
		profile = "default"
	}
	status := instance.GetStatus()
	statusStyled := status
	if status == "RUNNING" {
		statusStyled = s.UI.Status(status)
	}

	fmt.Fprintf(s.UI.Out, "%s (%s) %s\n", nameStyled, profile, statusStyled)
	if info, ok := autoTerminationDetails(instance, time.Now()); ok {
		remaining := s.UI.Highlight(formatDurationLong(info.Remaining))
		at := s.UI.Highlight(info.EndAt.Local().Format("15:04pm"))
		s.UI.Infof("%s %s (at %s)", info.Prefix, remaining, at)
	}
	s.UI.Infof("Zone: %s", s.Config.Project.Zone)

	machineName := gcp.ShortName(instance.GetMachineType())
	if machineName == "" {
		machineName = instance.GetMachineType()
	}
	s.UI.Infof("Machine: %s", machineName)

	specs, err := s.machineSpecs(ctx, machineName, instance)
	if err != nil {
		s.Logger.Debug("machine specs unavailable", zap.Error(err))
	} else {
		for _, spec := range specs {
			s.UI.InfofIndent(1, "%s", spec)
		}
	}
	if externalIP != "" {
		s.UI.Infof("External IP: %s", externalIP)
	}

	return nil
}

func (s *Service) machineSpecs(ctx context.Context, machineName string, instance *computepb.Instance) ([]string, error) {
	specs := []string{}
	if machineName == "" {
		return specs, nil
	}

	call := s.api("compute.machineTypes.get", gcp.ZoneResource(s.Config.Project.ID, s.Config.Project.Zone, "machineTypes", machineName), "")
	mt, err := s.Compute.GetMachineType(ctx, &computepb.GetMachineTypeRequest{
		Project:     s.Config.Project.ID,
		Zone:        s.Config.Project.Zone,
		MachineType: machineName,
	})
	call.Stop()
	if err != nil {
		return specs, err
	}

	cpus := mt.GetGuestCpus()
	arch := mt.GetArchitecture()
	if cpus > 0 {
		if arch != "" {
			specs = append(specs, fmt.Sprintf("CPUs: %d (%s)", cpus, arch))
		} else {
			specs = append(specs, fmt.Sprintf("CPUs: %d", cpus))
		}
	}

	if mem := mt.GetMemoryMb(); mem > 0 {
		specs = append(specs, fmt.Sprintf("RAM: %s", formatGiB(mem)))
	}

	gpuType, gpuCount := gpuFromInstance(instance)
	if gpuCount > 0 {
		specs = append(specs, fmt.Sprintf("GPU: %s x%d", gpuType, gpuCount))
	}

	return specs, nil
}

func gpuFromInstance(instance *computepb.Instance) (string, int32) {
	if instance == nil {
		return "", 0
	}
	var count int32
	gpuType := ""
	for _, accel := range instance.GetGuestAccelerators() {
		count += accel.GetAcceleratorCount()
		if gpuType == "" {
			gpuType = gcp.ShortName(accel.GetAcceleratorType())
		}
	}
	return gpuType, count
}

func formatGiB(memoryMb int32) string {
	gb := float64(memoryMb) / 1024.0
	if math.Abs(gb-math.Round(gb)) < 0.01 {
		return fmt.Sprintf("%.0f GB", gb)
	}
	return fmt.Sprintf("%.1f GB", gb)
}

type autoTerminationInfo struct {
	Prefix    string
	Remaining time.Duration
	EndAt     time.Time
}

func autoTerminationDetails(instance *computepb.Instance, now time.Time) (autoTerminationInfo, bool) {
	if instance == nil || instance.GetStatus() != "RUNNING" {
		return autoTerminationInfo{}, false
	}
	scheduling := instance.GetScheduling()
	if scheduling == nil || scheduling.GetMaxRunDuration() == nil {
		return autoTerminationInfo{}, false
	}
	seconds := scheduling.GetMaxRunDuration().GetSeconds()
	if seconds <= 0 {
		return autoTerminationInfo{}, false
	}
	lastStart := instance.GetLastStartTimestamp()
	if lastStart == "" {
		return autoTerminationInfo{}, false
	}
	startAt, err := time.Parse(time.RFC3339Nano, lastStart)
	if err != nil {
		return autoTerminationInfo{}, false
	}
	endAt := startAt.Add(time.Duration(seconds) * time.Second)
	remaining := endAt.Sub(now)
	if remaining <= 0 {
		return autoTerminationInfo{}, false
	}

	action := strings.ToUpper(scheduling.GetInstanceTerminationAction())
	prefix := "Auto-terminating in"
	if action == "STOP" {
		prefix = "Auto-stopping in"
	} else if action == "DELETE" {
		prefix = "Auto-terminating in"
	}

	return autoTerminationInfo{Prefix: prefix, Remaining: remaining, EndAt: endAt}, true
}

func formatDurationLong(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	totalMinutes := int(d.Truncate(time.Minute).Minutes())
	days := totalMinutes / (60 * 24)
	hours := (totalMinutes / 60) % 24
	mins := totalMinutes % 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if len(parts) == 0 {
		return "<1m"
	}
	return strings.Join(parts, " ")
}

func (s *Service) getInstance(ctx context.Context, name string) (*computepb.Instance, error) {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	call := s.api("compute.instances.get", gcp.ZoneResource(project, zone, "instances", name), "")
	instance, err := s.Compute.GetInstance(ctx, &computepb.GetInstanceRequest{
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
	return instance, nil
}

func (s *Service) ensureFirewall(ctx context.Context, name string, ports []int) error {
	project := s.Config.Project.ID
	ruleName := fmt.Sprintf("%s-ports", name)
	network := gcp.GlobalResource(project, "networks", s.Config.Network.DefaultNetwork)
	allowed := portsToAllowed(ports)
	portTag := ruleName

	rule := &computepb.Firewall{
		Name:         proto.String(ruleName),
		Network:      proto.String(network),
		Direction:    proto.String("INGRESS"),
		Allowed:      allowed,
		TargetTags:   []string{portTag},
		SourceRanges: []string{"0.0.0.0/0"},
	}

	getCall := s.api("compute.firewalls.get", gcp.GlobalResource(project, "firewalls", ruleName), "")
	_, err := s.Compute.GetFirewall(ctx, &computepb.GetFirewallRequest{
		Project:  project,
		Firewall: ruleName,
	})
	getCall.Stop()
	if err != nil {
		if gcp.IsNotFound(err) {
			call := s.api("compute.firewalls.insert", gcp.GlobalResource(project, "firewalls", ruleName), fmt.Sprintf("Updating firewall %s", ruleName))
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

	call := s.api("compute.firewalls.patch", gcp.GlobalResource(project, "firewalls", ruleName), fmt.Sprintf("Updating firewall %s", ruleName))
	op, err := s.Compute.PatchFirewall(ctx, &computepb.PatchFirewallRequest{
		Project:          project,
		Firewall:         ruleName,
		FirewallResource: rule,
	})
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) ensureTags(ctx context.Context, instance *computepb.Instance, name string) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	portTag := fmt.Sprintf("%s-ports", name)
	tags := append([]string{}, s.Config.Network.TagsBase...)
	tags = append(tags, portTag)

	existing := instance.GetTags().GetItems()
	if containsAll(existing, tags) {
		return nil
	}

	req := &computepb.SetTagsInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
		TagsResource: &computepb.Tags{
			Items:       tags,
			Fingerprint: proto.String(instance.GetTags().GetFingerprint()),
		},
	}
	call := s.api("compute.instances.setTags", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Updating tags for %s", name))
	op, err := s.Compute.SetInstanceTags(ctx, req)
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) setMaxRunDuration(ctx context.Context, name string, maxHours int) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	scheduling := s.Builder.Scheduling(maxHours)
	call := s.api("compute.instances.setScheduling", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Updating scheduling for %s", name))
	op, err := s.Compute.SetInstanceScheduling(ctx, &computepb.SetSchedulingInstanceRequest{
		Project:            project,
		Zone:               zone,
		Instance:           name,
		SchedulingResource: scheduling,
	})
	if err != nil {
		call.Stop()
		return err
	}
	return s.wait(ctx, call, op)
}

func (s *Service) startInstance(ctx context.Context, name string) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	call := s.api("compute.instances.start", gcp.ZoneResource(project, zone, "instances", name), fmt.Sprintf("Starting %s", name))
	op, err := s.Compute.StartInstance(ctx, &computepb.StartInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: name,
	})
	if err != nil {
		call.Stop()
		return err
	}
	if err := s.wait(ctx, call, op); err != nil {
		return err
	}
	s.UI.Successf("Started %s", name)
	return nil
}

func (s *Service) setAutoDelete(ctx context.Context, name string, instance *computepb.Instance, autoDelete bool) error {
	project := s.Config.Project.ID
	zone := s.Config.Project.Zone

	for _, disk := range instance.GetDisks() {
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

func portsToAllowed(ports []int) []*computepb.Allowed {
	var portStrings []string
	for _, port := range ports {
		portStrings = append(portStrings, strconv.Itoa(port))
	}
	return []*computepb.Allowed{{
		IPProtocol: proto.String("tcp"),
		Ports:      portStrings,
	}}
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
