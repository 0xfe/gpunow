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
