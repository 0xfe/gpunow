package cli

import "testing"

func TestSSHCommandArgsDefaultTarget(t *testing.T) {
	args := sshCommandArgs([]string{}, false)
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
	args = sshCommandArgs([]string{"nvidia-smi"}, false)
	if len(args) != 1 || args[0] != "nvidia-smi" {
		t.Fatalf("expected passthrough args, got %v", args)
	}
}

func TestSSHCommandArgsWithTarget(t *testing.T) {
	args := sshCommandArgs([]string{"gpu0"}, true)
	if len(args) != 0 {
		t.Fatalf("expected no command args, got %v", args)
	}
	args = sshCommandArgs([]string{"gpu0", "nvidia-smi", "--query"}, true)
	if len(args) != 2 || args[0] != "nvidia-smi" || args[1] != "--query" {
		t.Fatalf("unexpected args: %v", args)
	}
}
