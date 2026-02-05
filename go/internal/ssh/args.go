package ssh

import "strings"

type SSHOptions struct {
	User         string
	Host         string
	ProxyJump    string
	ForwardAgent bool
	Command      []string
}

type SCPOptions struct {
	ProxyJump string
	Src       string
	Dst       string
}

func BuildSSHArgs(opts SSHOptions) []string {
	args := []string{}
	if opts.ForwardAgent {
		args = append(args, "-A")
	}
	if opts.ProxyJump != "" {
		args = append(args, "-J", opts.ProxyJump)
	}
	target := opts.Host
	if opts.User != "" {
		target = opts.User + "@" + opts.Host
	}
	args = append(args, target)
	args = append(args, opts.Command...)
	return args
}

func BuildSCPArgs(opts SCPOptions) []string {
	args := []string{}
	if opts.ProxyJump != "" {
		args = append(args, "-o", "ProxyJump="+opts.ProxyJump)
	}
	args = append(args, opts.Src, opts.Dst)
	return args
}

func FormatUserHost(user, host string) string {
	if user == "" {
		return host
	}
	return user + "@" + host
}

func NormalizePath(path string) string {
	return strings.TrimSpace(path)
}
