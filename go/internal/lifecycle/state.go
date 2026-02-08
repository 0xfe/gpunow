package lifecycle

import "strings"

const (
	InstanceStateTerminated   = "TERMINATED"
	InstanceStateStarting     = "STARTING"
	InstanceStateProvisioning = "PROVISIONING"
	InstanceStateReady        = "READY"
	InstanceStateTerminating  = "TERMINATING"
)

func NormalizeInstanceState(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case InstanceStateTerminated:
		return InstanceStateTerminated
	case InstanceStateStarting:
		return InstanceStateStarting
	case InstanceStateProvisioning:
		return InstanceStateProvisioning
	case InstanceStateReady:
		return InstanceStateReady
	case InstanceStateTerminating:
		return InstanceStateTerminating
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func FromComputeStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "TERMINATED":
		return InstanceStateTerminated
	case "STOPPING", "SUSPENDING":
		return InstanceStateTerminating
	case "PROVISIONING", "STAGING":
		return InstanceStateStarting
	case "RUNNING":
		return InstanceStateReady
	default:
		return NormalizeInstanceState(value)
	}
}
