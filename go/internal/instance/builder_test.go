package instance

import "testing"

func TestDiskMode(t *testing.T) {
	cases := map[string]string{
		"rw":         "READ_WRITE",
		"read_only":  "READ_ONLY",
		"READ_WRITE": "READ_WRITE",
	}
	for input, expected := range cases {
		if got := diskMode(input); got != expected {
			t.Fatalf("diskMode(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestReservationAffinityType(t *testing.T) {
	cases := map[string]string{
		"":         "NO_RESERVATION",
		"none":     "NO_RESERVATION",
		"any":      "ANY_RESERVATION",
		"specific": "SPECIFIC_RESERVATION",
	}
	for input, expected := range cases {
		if got := reservationAffinityType(input); got != expected {
			t.Fatalf("reservationAffinityType(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestKeyRevocationActionType(t *testing.T) {
	cases := map[string]string{
		"":     "NONE",
		"none": "NONE",
		"stop": "STOP",
	}
	for input, expected := range cases {
		if got := keyRevocationActionType(input); got != expected {
			t.Fatalf("keyRevocationActionType(%q) = %q, want %q", input, got, expected)
		}
	}
}
