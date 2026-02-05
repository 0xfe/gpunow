package vm

import (
	"testing"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

func TestAutoTerminationInfo(t *testing.T) {
	now := time.Date(2026, 2, 5, 22, 0, 0, 0, time.UTC)
	start := now.Add(-90 * time.Minute)

	instance := &computepb.Instance{
		Status:             proto.String("RUNNING"),
		LastStartTimestamp: proto.String(start.Format(time.RFC3339Nano)),
		Scheduling: &computepb.Scheduling{
			MaxRunDuration:            &computepb.Duration{Seconds: proto.Int64(4 * 3600)},
			InstanceTerminationAction: proto.String("DELETE"),
		},
	}

	info, ok := autoTerminationDetails(instance, now)
	if !ok {
		t.Fatalf("expected auto-termination info")
	}
	if info.Prefix != "Auto-terminating in" {
		t.Fatalf("unexpected prefix: %s", info.Prefix)
	}
	if got := formatDurationLong(info.Remaining); got != "2h 30m" {
		t.Fatalf("unexpected remaining: %s", got)
	}
}

func TestAutoTerminationInfoNotRunning(t *testing.T) {
	instance := &computepb.Instance{
		Status: proto.String("TERMINATED"),
		Scheduling: &computepb.Scheduling{
			MaxRunDuration:            &computepb.Duration{Seconds: proto.Int64(3600)},
			InstanceTerminationAction: proto.String("DELETE"),
		},
	}

	if _, ok := autoTerminationDetails(instance, time.Now()); ok {
		t.Fatalf("did not expect message for non-running instance")
	}
}

func TestFormatDurationLong(t *testing.T) {
	if got := formatDurationLong(30 * time.Second); got != "<1m" {
		t.Fatalf("expected <1m, got %s", got)
	}
	if got := formatDurationLong(4*time.Hour + 24*time.Minute); got != "4h 24m" {
		t.Fatalf("expected 4h 24m, got %s", got)
	}
	if got := formatDurationLong(26*time.Hour + 5*time.Minute); got != "1d 2h 5m" {
		t.Fatalf("expected 1d 2h 5m, got %s", got)
	}
}
