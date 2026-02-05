package gcp

import (
	"fmt"
	"strings"
)

// RegionFromZone converts a zone like "us-east1-d" into "us-east1".
func RegionFromZone(zone string) (string, error) {
	parts := strings.Split(zone, "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid zone: %s", zone)
	}
	return strings.Join(parts[:2], "-"), nil
}
