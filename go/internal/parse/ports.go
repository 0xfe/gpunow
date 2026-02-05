package parse

import (
	"fmt"
	"strconv"
	"strings"
)

func PortsCSV(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("ports value is empty")
	}
	parts := strings.Split(value, ",")
	ports := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil || p <= 0 || p > 65535 {
			return nil, fmt.Errorf("invalid port: %s", part)
		}
		ports = append(ports, p)
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no valid ports found")
	}
	return ports, nil
}
