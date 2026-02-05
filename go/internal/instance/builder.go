package instance

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"

	"gpunow/internal/config"
	"gpunow/internal/gcp"
)

type Builder struct {
	Config *config.Config
}

type Options struct {
	Name        string
	Network     string
	Subnetwork  string
	PublicIP    bool
	Tags        []string
	CloudInit   string
	MaxRunHours int
	Labels      map[string]string
	Metadata    map[string]string
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

	disk, err := b.buildBootDisk(ctx, compute, opts.Name)
	if err != nil {
		return nil, err
	}

	machineType := gcp.ZoneResource(project, zone, "machineTypes", b.Config.Instance.MachineType)
	acceleratorType := gcp.ZoneResource(project, zone, "acceleratorTypes", b.Config.GPU.Type)

	scheduling := b.buildScheduling(maxHours)

	tags := opts.Tags
	if len(tags) == 0 {
		tags = append([]string{}, b.Config.Network.TagsBase...)
	}

	labels := mergeLabels(b.Config.Labels, opts.Labels)
	metadataItems := buildMetadataItems(b.Config.Metadata, opts.Metadata, opts.CloudInit)

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
		ServiceAccounts: []*computepb.ServiceAccount{
			{
				Email:  proto.String(b.Config.ServiceAccount.Email),
				Scopes: b.Config.ServiceAccount.Scopes,
			},
		},
		GuestAccelerators: []*computepb.AcceleratorConfig{
			{
				AcceleratorCount: proto.Int32(int32(b.Config.GPU.Count)),
				AcceleratorType:  proto.String(acceleratorType),
			},
		},
		Tags: &computepb.Tags{
			Items: tags,
		},
		Metadata: &computepb.Metadata{Items: metadataItems},
		ShieldedInstanceConfig: &computepb.ShieldedInstanceConfig{
			EnableSecureBoot:          proto.Bool(b.Config.Shielded.SecureBoot),
			EnableVtpm:                proto.Bool(b.Config.Shielded.VTPM),
			EnableIntegrityMonitoring: proto.Bool(b.Config.Shielded.IntegrityMonitoring),
		},
		Labels: labels,
		ReservationAffinity: &computepb.ReservationAffinity{
			ConsumeReservationType: proto.String(reservationAffinityType(b.Config.Reservation.Affinity)),
		},
		KeyRevocationActionType: proto.String(keyRevocationActionType(b.Config.Instance.KeyRevocationAction)),
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
	return b.buildScheduling(maxHours)
}

func (b *Builder) buildScheduling(maxHours int) *computepb.Scheduling {
	duration := &computepb.Duration{Seconds: proto.Int64(int64(maxHours) * 3600)}

	return &computepb.Scheduling{
		ProvisioningModel:         proto.String(b.Config.Instance.ProvisioningModel),
		OnHostMaintenance:         proto.String(b.Config.Instance.MaintenancePolicy),
		InstanceTerminationAction: proto.String(b.Config.Instance.TerminationAction),
		AutomaticRestart:          proto.Bool(b.Config.Instance.RestartOnFailure),
		MaxRunDuration:            duration,
	}
}

func (b *Builder) buildBootDisk(ctx context.Context, compute gcp.Compute, name string) (*computepb.AttachedDisk, error) {
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
			AutoDelete: proto.Bool(b.Config.Disk.AutoDelete),
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
		DiskSizeGb:  proto.Int64(int64(b.Config.Disk.SizeGB)),
		DiskType:    proto.String(diskType),
		SourceImage: proto.String(b.Config.Disk.Image),
	}

	return &computepb.AttachedDisk{
		AutoDelete:       proto.Bool(b.Config.Disk.AutoDelete),
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

func buildMetadataItems(base map[string]string, override map[string]string, cloudInit string) []*computepb.Items {
	merged := map[string]string{}
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}

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
