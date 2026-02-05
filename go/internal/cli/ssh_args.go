package cli

func sshCommandArgs(args []string, targetProvided bool) []string {
	if !targetProvided {
		return args
	}
	if len(args) <= 1 {
		return []string{}
	}
	return args[1:]
}
