package target

import "testing"

func TestParseName(t *testing.T) {
	parsed, err := Parse("gpu0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.IsCluster {
		t.Fatalf("expected non-cluster target")
	}
	if parsed.Name != "gpu0" {
		t.Fatalf("unexpected name: %s", parsed.Name)
	}
}

func TestParseClusterRef(t *testing.T) {
	parsed, err := Parse("my-cluster/0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !parsed.IsCluster {
		t.Fatalf("expected cluster target")
	}
	if parsed.Cluster != "my-cluster" || parsed.Index != 0 {
		t.Fatalf("unexpected cluster parse: %+v", parsed)
	}
	if parsed.Name != "my-cluster-0" {
		t.Fatalf("unexpected name: %s", parsed.Name)
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []string{"bad/", "bad/abc", "Bad/0", "bad/1/2", "/0"}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Fatalf("expected error for %s", c)
		}
	}
}
