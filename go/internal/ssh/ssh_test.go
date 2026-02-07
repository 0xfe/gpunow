package ssh

import "testing"

func TestBuildSSHArgs(t *testing.T) {
	args := BuildSSHArgs(SSHOptions{
		User:         "mo",
		Host:         "1.2.3.4",
		ProxyJump:    "mo@5.6.7.8",
		ForwardAgent: true,
		IdentityFile: "/home/mo/.ssh/id_ed25519",
		Command:      []string{"nvidia-smi"},
	})
	joined := join(args)
	if joined != "-i /home/mo/.ssh/id_ed25519 -A -J mo@5.6.7.8 mo@1.2.3.4 nvidia-smi" {
		t.Fatalf("unexpected args: %s", joined)
	}
}

func TestBuildSCPArgs(t *testing.T) {
	args := BuildSCPArgs(SCPOptions{
		ProxyJump:    "mo@5.6.7.8",
		IdentityFile: "/home/mo/.ssh/id_ed25519",
		Src:          "./local.txt",
		Dst:          "mo@1.2.3.4:/home/mo/",
	})
	joined := join(args)
	if joined != "-i /home/mo/.ssh/id_ed25519 -o ProxyJump=mo@5.6.7.8 ./local.txt mo@1.2.3.4:/home/mo/" {
		t.Fatalf("unexpected args: %s", joined)
	}
}

func TestParseRemoteSpec(t *testing.T) {
	local, err := ParseRemoteSpec("./local.txt")
	if err != nil {
		t.Fatalf("parse local: %v", err)
	}
	if local.IsRemote {
		t.Fatalf("expected local spec")
	}

	remote, err := ParseRemoteSpec("my-cluster/2:/home/mo/")
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	if !remote.IsRemote || remote.Target.Cluster != "my-cluster" || remote.Target.Index != 2 {
		t.Fatalf("unexpected remote spec: %+v", remote)
	}
}

func join(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += " "
		}
		out += part
	}
	return out
}
