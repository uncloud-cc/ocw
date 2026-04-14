package cli

import "strings"

// reorderArgs reorders command line arguments so that flags come before positional args.
// This allows flags to be placed anywhere: ocw myjob --show-secrets works the same as ocw --show-secrets myjob
func reorderArgs(args []string) []string {
	var flags []string
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		// Check if it's a flag (starts with -)
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Check if this flag takes a value (next arg doesn't start with -)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
		i++
	}

	return append(flags, positional...)
}
