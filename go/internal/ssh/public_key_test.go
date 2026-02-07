package ssh

import "testing"

func TestCanonicalizePublicKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "ed25519 keeps type and key",
			input:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyForTest user@host",
			want:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyForTest",
			wantErr: false,
		},
		{
			name:    "security key with colon comment",
			input:   "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTY ssh:",
			want:    "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTY",
			wantErr: false,
		},
		{
			name:    "invalid",
			input:   "not-a-key",
			want:    "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalizePublicKey(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("canonicalizePublicKey() err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("canonicalizePublicKey() = %q, want %q", got, tc.want)
			}
		})
	}
}
