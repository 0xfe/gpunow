package labels

import "fmt"

const (
	ManagedKey   = "gpunow"
	ManagedValue = "0xfe"
)

func EnsureManaged(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	out[ManagedKey] = ManagedValue
	return out
}

func Filter() string {
	return fmt.Sprintf("labels.%s = %q", ManagedKey, ManagedValue)
}
