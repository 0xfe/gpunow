package instance

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"

	"gpunow/internal/config"
	"gpunow/internal/gcp"
	"gpunow/internal/labels"
)

type Builder struct {
	Config *config.Config
}

type Options struct {
	Name              string
	Network           string
	Subnetwork        string
	PublicIP          bool
	Tags              []string
	CloudInit         string
	MachineType       string
	MaxRunHours       int
	TerminationAction string
	DiskSizeGB        int
	DiskAutoDelete    *bool
	Labels            map[string]string
	Metadata          map[string]string
}

func NewBuilder(cfg *config.Config) *Builder {
	return &Builder{Config: cfg}
}

func (b *Builder) Build(ctx context.Context, compute gcp.Compute, opts Options) (*computepb.InsertInstanceRequest, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("instance name is required")
	}
	if opts.Network == "" {
		return nil, fmt.Errorf("network is required")
	}
	if opts.CloudInit == "" {
		return nil, fmt.Errorf("cloud-init content is required")
	}
	project := b.Config.Project.ID
	zone := b.Config.Project.Zone

	maxHours := opts.MaxRunHours
	if maxHours <= 0 {
		maxHours = b.Config.Instance.MaxRunHours
	}
	machineTypeName := strings.TrimSpace(opts.MachineType)
	if machineTypeName == "" {
		machineTypeName = b.Config.Instance.MachineType
	}
	terminationAction := strings.TrimSpace(opts.TerminationAction)
	if terminationAction == "" {
		terminationAction = b.Config.Instance.TerminationAction
	}
	diskSizeGB := opts.DiskSizeGB
	if diskSizeGB <= 0 {
		diskSizeGB = b.Config.Disk.SizeGB
	}
	diskAutoDelete := b.Config.Disk.AutoDelete
	if opts.DiskAutoDelete != nil {
		diskAutoDelete = *opts.DiskAutoDelete
	}

	mergedLabels := labels.EnsureManaged(mergeLabels(nil, opts.Labels))

	disk, err := b.buildBootDisk(ctx, compute, opts.Name, mergedLabels, diskSizeGB, diskAutoDelete)
	if err != nil {
		return nil, err
	}

	machineType := gcp.ZoneResource(project, zone, "machineTypes", machineTypeName)

	scheduling := b.buildScheduling(maxHours, terminationAction)

	tags := opts.Tags
	if len(tags) == 0 {
		tags = append([]string{}, b.Config.Network.TagsBase...)
	}

	metadata := mergeMetadata(b.Config.Metadata, opts.Metadata)
	metadataItems := buildMetadataItems(metadata, opts.CloudInit)

	iface := &computepb.NetworkInterface{
		Network:   proto.String(opts.Network),
		StackType: proto.String(b.Config.Network.StackType),
	}
	if opts.Subnetwork != "" {
		iface.Subnetwork = proto.String(opts.Subnetwork)
	}
	if opts.PublicIP {
		iface.AccessConfigs = []*computepb.AccessConfig{
			{
				Name:        proto.String("External NAT"),
				Type:        proto.String("ONE_TO_ONE_NAT"),
				NetworkTier: proto.String(b.Config.Network.NetworkTier),
			},
		}
	}

	instance := &computepb.Instance{
		Name:              proto.String(opts.Name),
		MachineType:       proto.String(machineType),
		Disks:             []*computepb.AttachedDisk{disk},
		NetworkInterfaces: []*computepb.NetworkInterface{iface},
		Scheduling:        scheduling,
		Tags: &computepb.Tags{
			Items: tags,
		},
		Metadata: &computepb.Metadata{Items: metadataItems},
		ShieldedInstanceConfig: &computepb.ShieldedInstanceConfig{
			EnableSecureBoot:          proto.Bool(b.Config.Shielded.SecureBoot),
			EnableVtpm:                proto.Bool(b.Config.Shielded.VTPM),
			EnableIntegrityMonitoring: proto.Bool(b.Config.Shielded.IntegrityMonitoring),
		},
		Labels: mergedLabels,
		ReservationAffinity: &computepb.ReservationAffinity{
			ConsumeReservationType: proto.String(reservationAffinityType(b.Config.Reservation.Affinity)),
		},
		KeyRevocationActionType: proto.String(keyRevocationActionType(b.Config.Instance.KeyRevocationAction)),
	}
	if email := strings.TrimSpace(b.Config.ServiceAccount.Email); email != "" && len(b.Config.ServiceAccount.Scopes) > 0 {
		instance.ServiceAccounts = []*computepb.ServiceAccount{{
			Email:  proto.String(email),
			Scopes: b.Config.ServiceAccount.Scopes,
		}}
	}
	if hostname := b.hostname(opts.Name); hostname != "" {
		instance.Hostname = proto.String(hostname)
	}

	return &computepb.InsertInstanceRequest{
		Project:          project,
		Zone:             zone,
		InstanceResource: instance,
	}, nil
}

func (b *Builder) hostname(name string) string {
	domain := strings.TrimSpace(b.Config.Instance.HostnameDomain)
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s", name, strings.TrimPrefix(domain, "."))
}

func (b *Builder) Scheduling(maxHours int) *computepb.Scheduling {
	return b.buildScheduling(maxHours, b.Config.Instance.TerminationAction)
}

func (b *Builder) buildScheduling(maxHours int, terminationAction string) *computepb.Scheduling {
	duration := &computepb.Duration{Seconds: proto.Int64(int64(maxHours) * 3600)}

	return &computepb.Scheduling{
		ProvisioningModel:         proto.String(b.Config.Instance.ProvisioningModel),
		OnHostMaintenance:         proto.String(b.Config.Instance.MaintenancePolicy),
		InstanceTerminationAction: proto.String(terminationAction),
		AutomaticRestart:          proto.Bool(b.Config.Instance.RestartOnFailure),
		MaxRunDuration:            duration,
	}
}

func (b *Builder) buildBootDisk(ctx context.Context, compute gcp.Compute, name string, diskLabels map[string]string, diskSizeGB int, autoDelete bool) (*computepb.AttachedDisk, error) {
	project := b.Config.Project.ID
	zone := b.Config.Project.Zone

	diskReq := &computepb.GetDiskRequest{
		Project: project,
		Zone:    zone,
		Disk:    name,
	}
	disk, err := compute.GetDisk(ctx, diskReq)
	if err == nil && disk != nil {
		return &computepb.AttachedDisk{
			AutoDelete: proto.Bool(autoDelete),
			Boot:       proto.Bool(b.Config.Disk.Boot),
			DeviceName: proto.String(name),
			Mode:       proto.String(diskMode(b.Config.Disk.Mode)),
			Source:     proto.String(gcp.ZoneResource(project, zone, "disks", name)),
		}, nil
	}
	if err != nil && !gcp.IsNotFound(err) {
		return nil, err
	}

	diskType := gcp.ZoneResource(project, zone, "diskTypes", b.Config.Disk.Type)
	initParams := &computepb.AttachedDiskInitializeParams{
		DiskName:    proto.String(name),
		DiskSizeGb:  proto.Int64(int64(diskSizeGB)),
		DiskType:    proto.String(diskType),
		SourceImage: proto.String(b.Config.Disk.Image),
	}
	if len(diskLabels) > 0 {
		initParams.Labels = diskLabels
	}

	return &computepb.AttachedDisk{
		AutoDelete:       proto.Bool(autoDelete),
		Boot:             proto.Bool(b.Config.Disk.Boot),
		DeviceName:       proto.String(name),
		Mode:             proto.String(diskMode(b.Config.Disk.Mode)),
		InitializeParams: initParams,
	}, nil
}

func diskMode(value string) string {
	switch strings.ToLower(value) {
	case "rw", "read_write", "read-write", "readwrite":
		return "READ_WRITE"
	case "ro", "read_only", "read-only", "readonly":
		return "READ_ONLY"
	default:
		return value
	}
}

func reservationAffinityType(value string) string {
	switch strings.ToLower(value) {
	case "", "none", "no_reservation", "no-reservation":
		return "NO_RESERVATION"
	case "any", "any_reservation", "any-reservation":
		return "ANY_RESERVATION"
	case "specific", "specific_reservation", "specific-reservation":
		return "SPECIFIC_RESERVATION"
	default:
		return strings.ToUpper(value)
	}
}

func keyRevocationActionType(value string) string {
	switch strings.ToLower(value) {
	case "", "none":
		return "NONE"
	case "stop":
		return "STOP"
	default:
		return strings.ToUpper(value)
	}
}

func mergeLabels(base map[string]string, override map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

func buildMetadataItems(merged map[string]string, cloudInit string) []*computepb.Items {
	items := []*computepb.Items{{
		Key:   proto.String("user-data"),
		Value: proto.String(cloudInit),
	}}

	for k, v := range merged {
		key := k
		val := v
		items = append(items, &computepb.Items{Key: &key, Value: &val})
	}
	return items
}

func mergeMetadata(base map[string]string, override map[string]string) map[string]string {
	merged := map[string]string{}
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		if k == "ssh-keys" {
			merged[k] = appendMetadataLines(merged[k], v)
			continue
		}
		merged[k] = v
	}
	return merged
}

func appendMetadataLines(existing, extra string) string {
	existing = strings.TrimSpace(existing)
	extra = strings.TrimSpace(extra)
	if existing == "" {
		return extra
	}
	if extra == "" {
		return existing
	}
	return existing + "\n" + extra
}
