package cluster

import (
	"net"
	"testing"
)

func TestDeriveSubnetCIDR(t *testing.T) {
	cidr, err := DeriveSubnetCIDR("10.200.0.0/16", 24, "my-cluster")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse derived cidr: %v", err)
	}
	_, base, _ := net.ParseCIDR("10.200.0.0/16")
	if !base.Contains(subnet.IP) {
		t.Fatalf("derived subnet not within base: %s", cidr)
	}
}
