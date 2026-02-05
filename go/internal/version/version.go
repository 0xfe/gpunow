package version

import "fmt"

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func String() string {
	return fmt.Sprintf("%s (%s, %s)", Version, Commit, BuildTime)
}
