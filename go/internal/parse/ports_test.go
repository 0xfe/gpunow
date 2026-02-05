package parse

import "testing"

func TestPortsCSV(t *testing.T) {
	ports, err := PortsCSV("22, 80,443")
	if err != nil {
		t.Fatalf("parse ports: %v", err)
	}
	if len(ports) != 3 || ports[0] != 22 || ports[1] != 80 || ports[2] != 443 {
		t.Fatalf("unexpected ports: %v", ports)
	}
}

func TestPortsCSVInvalid(t *testing.T) {
	cases := []string{"", "0", "70000", "abc"}
	for _, c := range cases {
		if _, err := PortsCSV(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}
