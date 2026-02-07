package ssh

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"

	"gpunow/internal/config"
	"gpunow/internal/gcp"
)

func EnsureInstanceSSHKey(ctx context.Context, compute gcp.Compute, cfg *config.Config, instanceName, user, publicKey string) error {
	user = strings.TrimSpace(user)
	publicKey = strings.TrimSpace(publicKey)
	if instanceName == "" || user == "" || publicKey == "" {
		return nil
	}

	inst, err := compute.GetInstance(ctx, &computepb.GetInstanceRequest{
		Project:  cfg.Project.ID,
		Zone:     cfg.Project.Zone,
		Instance: instanceName,
	})
	if err != nil {
		return err
	}

	meta := inst.GetMetadata()
	if meta == nil {
		meta = &computepb.Metadata{}
	}
	items := meta.GetItems()
	line := fmt.Sprintf("%s:%s", user, publicKey)

	found := false
	osLoginSet := false
	needsUpdate := false
	for _, item := range items {
		switch item.GetKey() {
		case "ssh-keys":
			if sshKeyPresent(item.GetValue(), line) {
				found = true
				continue
			}
			updated := appendSSHKey(item.GetValue(), line)
			item.Value = proto.String(updated)
			found = true
			needsUpdate = true
		case "enable-oslogin":
			osLoginSet = true
			if strings.EqualFold(strings.TrimSpace(item.GetValue()), "false") {
				continue
			}
			item.Value = proto.String("FALSE")
			needsUpdate = true
		default:
			continue
		}
	}

	if !found {
		items = append(items, &computepb.Items{
			Key:   proto.String("ssh-keys"),
			Value: proto.String(line),
		})
		needsUpdate = true
	}
	if !osLoginSet {
		items = append(items, &computepb.Items{
			Key:   proto.String("enable-oslogin"),
			Value: proto.String("FALSE"),
		})
		needsUpdate = true
	}
	if !needsUpdate {
		return nil
	}

	req := &computepb.SetMetadataInstanceRequest{
		Project:  cfg.Project.ID,
		Zone:     cfg.Project.Zone,
		Instance: instanceName,
		MetadataResource: &computepb.Metadata{
			Fingerprint: meta.Fingerprint,
			Items:       items,
		},
	}
	op, err := compute.SetInstanceMetadata(ctx, req)
	if err != nil {
		return err
	}
	return op.Wait(ctx)
}

func sshKeyPresent(existing, line string) bool {
	for _, entry := range strings.Split(existing, "\n") {
		if strings.TrimSpace(entry) == line {
			return true
		}
	}
	return false
}

func appendSSHKey(existing, line string) string {
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return line
	}
	return existing + "\n" + line
}
