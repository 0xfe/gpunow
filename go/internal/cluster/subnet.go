package cluster

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net"
)

func DeriveSubnetCIDR(baseCIDR string, prefix int, clusterName string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(baseCIDR)
	if err != nil {
		return "", fmt.Errorf("invalid base CIDR: %w", err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return "", fmt.Errorf("base CIDR must be IPv4")
	}
	basePrefix, _ := ipNet.Mask.Size()
	if prefix < basePrefix {
		return "", fmt.Errorf("subnet prefix %d is smaller than base prefix %d", prefix, basePrefix)
	}
	if prefix > 30 {
		return "", fmt.Errorf("subnet prefix must be <= 30")
	}

	count := 1 << (prefix - basePrefix)
	h := fnv.New32a()
	_, _ = h.Write([]byte(clusterName))
	index := int(h.Sum32() % uint32(count))

	base := binary.BigEndian.Uint32(ip4)
	blockSize := uint32(1) << (32 - prefix)
	subnetIP := base + (uint32(index) * blockSize)

	out := make(net.IP, 4)
	binary.BigEndian.PutUint32(out, subnetIP)
	return fmt.Sprintf("%s/%d", out.String(), prefix), nil
}
