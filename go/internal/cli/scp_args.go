package cli

import (
	"fmt"
	"strings"
)

var scpFlagsWithValue = map[string]bool{
	"-P": true,
	"-o": true,
	"-i": true,
	"-S": true,
	"-J": true,
	"-F": true,
	"-c": true,
	"-l": true,
}

func parseScpArgs(args []string) (flags []string, src string, dst string, err error) {
	if len(args) == 0 {
		return nil, "", "", fmt.Errorf("src and dst are required")
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}

		flags = append(flags, arg)

		if len(arg) > 2 {
			base := arg[:2]
			if scpFlagsWithValue[base] {
				i++
				continue
			}
		}

		if scpFlagsWithValue[arg] {
			if i+1 >= len(args) {
				return nil, "", "", fmt.Errorf("flag %s requires a value", arg)
			}
			flags = append(flags, args[i+1])
			i += 2
			continue
		}

		i++
	}

	remaining := args[i:]
	if len(remaining) != 2 {
		return nil, "", "", fmt.Errorf("expected src and dst (use -- to separate flags from paths)")
	}

	return flags, remaining[0], remaining[1], nil
}
