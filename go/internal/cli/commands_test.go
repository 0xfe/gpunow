package cli

import "testing"

func TestParseIntFlagFromArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		value   int
		found   bool
		wantErr bool
	}{
		{name: "missing", args: []string{"foo"}, value: 0, found: false, wantErr: false},
		{name: "short separated", args: []string{"foo", "-n", "3"}, value: 3, found: true, wantErr: false},
		{name: "long separated", args: []string{"foo", "--num-instances", "4"}, value: 4, found: true, wantErr: false},
		{name: "long equals", args: []string{"foo", "--num-instances=5"}, value: 5, found: true, wantErr: false},
		{name: "short compact", args: []string{"foo", "-n6"}, value: 6, found: true, wantErr: false},
		{name: "invalid", args: []string{"foo", "-n", "x"}, value: 0, found: true, wantErr: true},
		{name: "missing value", args: []string{"foo", "-n"}, value: 0, found: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value, found, err := parseIntFlagFromArgs(tc.args, "-n", "--num-instances", "--num-instances must be a positive integer")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got=%v wantErr=%v", err, tc.wantErr)
			}
			if value != tc.value {
				t.Fatalf("value mismatch: got=%d want=%d", value, tc.value)
			}
			if found != tc.found {
				t.Fatalf("found mismatch: got=%v want=%v", found, tc.found)
			}
		})
	}
}

func TestParseStringFlagFromArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		value   string
		found   bool
		wantErr bool
	}{
		{name: "missing", args: []string{"foo"}, value: "", found: false, wantErr: false},
		{name: "separated", args: []string{"foo", "--gcp-machine-type", "g2-standard-16"}, value: "g2-standard-16", found: true, wantErr: false},
		{name: "equals", args: []string{"foo", "--gcp-machine-type=g2-standard-24"}, value: "g2-standard-24", found: true, wantErr: false},
		{name: "missing value", args: []string{"foo", "--gcp-machine-type"}, value: "", found: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value, found, err := parseStringFlagFromArgs(tc.args, "--gcp-machine-type")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got=%v wantErr=%v", err, tc.wantErr)
			}
			if value != tc.value {
				t.Fatalf("value mismatch: got=%q want=%q", value, tc.value)
			}
			if found != tc.found {
				t.Fatalf("found mismatch: got=%v want=%v", found, tc.found)
			}
		})
	}
}

func TestResolveStopKeepDisks(t *testing.T) {
	tests := []struct {
		name        string
		deleteFlag  bool
		keepFlag    bool
		deleteDisks bool
		defaultKeep bool
		wantKeep    bool
		wantErr     bool
	}{
		{
			name:        "default delete disks",
			deleteFlag:  true,
			defaultKeep: false,
			wantKeep:    false,
		},
		{
			name:        "default keep disks",
			deleteFlag:  true,
			defaultKeep: true,
			wantKeep:    true,
		},
		{
			name:        "override keep",
			deleteFlag:  true,
			keepFlag:    true,
			defaultKeep: false,
			wantKeep:    true,
		},
		{
			name:        "override delete",
			deleteFlag:  true,
			deleteDisks: true,
			defaultKeep: true,
			wantKeep:    false,
		},
		{
			name:        "conflicting flags",
			deleteFlag:  true,
			keepFlag:    true,
			deleteDisks: true,
			defaultKeep: true,
			wantErr:     true,
		},
		{
			name:        "keep without delete",
			keepFlag:    true,
			defaultKeep: false,
			wantErr:     true,
		},
		{
			name:        "delete-disks without delete",
			deleteDisks: true,
			defaultKeep: true,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveStopKeepDisks(tc.deleteFlag, tc.keepFlag, tc.deleteDisks, tc.defaultKeep)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got != tc.wantKeep {
				t.Fatalf("keep mismatch: got=%v want=%v", got, tc.wantKeep)
			}
		})
	}
}
