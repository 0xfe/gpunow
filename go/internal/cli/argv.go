package cli

import "strings"

var knownCommands = map[string]struct{}{
	"help":    {},
	"install": {},
	"create":  {},
	"start":   {},
	"stop":    {},
	"update":  {},
	"ssh":     {},
	"scp":     {},
	"status":  {},
	"state":   {},
	"version": {},
}

// NormalizeArgs rewrites convenience shorthand forms into explicit subcommands.
func NormalizeArgs(argv []string) []string {
	if len(argv) < 2 {
		return argv
	}
	if hasKnownCommand(argv[1:]) {
		return argv
	}
	if !looksLikeCreateShorthand(argv[1:]) {
		return argv
	}
	normalized := make([]string, 0, len(argv)+1)
	normalized = append(normalized, argv[0], "create")
	normalized = append(normalized, argv[1:]...)
	return normalized
}

func hasKnownCommand(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		_, ok := knownCommands[arg]
		return ok
	}
	return false
}

func looksLikeCreateShorthand(args []string) bool {
	hasCreateFlag := false
	hasClusterArg := false
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == "-n" || arg == "--num-instances":
			hasCreateFlag = true
			idx++
		case strings.HasPrefix(arg, "--num-instances="), strings.HasPrefix(arg, "-n"):
			hasCreateFlag = true
		case arg == "--start":
			hasCreateFlag = true
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			hasClusterArg = true
		}
	}
	return hasCreateFlag && hasClusterArg
}
