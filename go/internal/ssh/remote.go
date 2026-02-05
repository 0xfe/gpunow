package ssh

import (
	"fmt"
	"strings"

	"gpunow/internal/target"
)

type RemoteSpec struct {
	Raw      string
	Target   target.Target
	Path     string
	IsRemote bool
}

func ParseRemoteSpec(value string) (RemoteSpec, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return RemoteSpec{}, fmt.Errorf("path is required")
	}
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 1 {
		return RemoteSpec{Raw: value, Path: value, IsRemote: false}, nil
	}
	if parts[1] == "" {
		return RemoteSpec{}, fmt.Errorf("remote path is required")
	}

	tgt, err := target.Parse(parts[0])
	if err != nil {
		return RemoteSpec{}, err
	}
	if !tgt.IsCluster {
		return RemoteSpec{}, fmt.Errorf("remote target must be cluster/index")
	}
	return RemoteSpec{Raw: value, Target: tgt, Path: parts[1], IsRemote: true}, nil
}
