package target

import (
	"fmt"
	"strconv"
	"strings"

	"gpunow/internal/validate"
)

type Target struct {
	Raw       string
	Name      string
	Cluster   string
	Index     int
	IsCluster bool
}

func Parse(input string) (Target, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Target{}, fmt.Errorf("target is required")
	}

	parts := strings.Split(input, "/")
	if len(parts) == 1 {
		if cluster, index, ok := parseClusterAlias(input); ok {
			name := fmt.Sprintf("%s-%d", cluster, index)
			return Target{Raw: input, Name: name, Cluster: cluster, Index: index, IsCluster: true}, nil
		}
		if !validate.IsResourceName(input) {
			return Target{}, fmt.Errorf("invalid instance name: %s", input)
		}
		return Target{Raw: input, Name: input}, nil
	}
	if len(parts) != 2 {
		return Target{}, fmt.Errorf("invalid target format: %s", input)
	}

	cluster := parts[0]
	indexRaw := parts[1]
	if !validate.IsResourceName(cluster) {
		return Target{}, fmt.Errorf("invalid cluster name: %s", cluster)
	}

	index, err := strconv.Atoi(indexRaw)
	if err != nil || index < 0 {
		return Target{}, fmt.Errorf("invalid instance index: %s", indexRaw)
	}

	name := fmt.Sprintf("%s-%d", cluster, index)
	return Target{Raw: input, Name: name, Cluster: cluster, Index: index, IsCluster: true}, nil
}

func parseClusterAlias(input string) (string, int, bool) {
	dash := strings.LastIndex(input, "-")
	if dash <= 0 || dash >= len(input)-1 {
		return "", 0, false
	}
	cluster := input[:dash]
	indexRaw := input[dash+1:]
	if !validate.IsResourceName(cluster) {
		return "", 0, false
	}
	index, err := strconv.Atoi(indexRaw)
	if err != nil || index < 0 {
		return "", 0, false
	}
	return cluster, index, true
}
