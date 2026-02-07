package cli

import (
	"reflect"
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "explicit create untouched",
			in:   []string{"gpunow", "create", "--start", "-n", "3", "foo"},
			want: []string{"gpunow", "create", "--start", "-n", "3", "foo"},
		},
		{
			name: "shorthand rewrites to create",
			in:   []string{"gpunow", "-n", "3", "foo", "--start"},
			want: []string{"gpunow", "create", "-n", "3", "foo", "--start"},
		},
		{
			name: "shorthand with positional first rewrites to create",
			in:   []string{"gpunow", "foo", "-n", "3", "--start"},
			want: []string{"gpunow", "create", "foo", "-n", "3", "--start"},
		},
		{
			name: "known command remains",
			in:   []string{"gpunow", "start", "foo", "-n", "3"},
			want: []string{"gpunow", "start", "foo", "-n", "3"},
		},
		{
			name: "no create flags remains",
			in:   []string{"gpunow", "foo"},
			want: []string{"gpunow", "foo"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeArgs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeArgs mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}
