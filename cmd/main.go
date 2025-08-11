package main

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/truefoundry/elasti/operator"
	"github.com/truefoundry/elasti/resolver"
)

func main() {
	subCommands := map[string]func(){
		"operator": operator.Main,
		"resolver": resolver.Main,
	}

	subCommandNames := slices.Collect(maps.Keys(subCommands))

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Please provide one of subcommands: %v\n", subCommandNames)
		os.Exit(1)
	}

	subcommand := os.Args[1]
	os.Args = slices.Delete(os.Args, 1, 1)

	switch subcommand {
	case "operator":
		operator.Main()
	case "resolver":
		resolver.Main()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %v", subcommand)
		fmt.Fprintf(os.Stderr, "Available subcommands: %v", subCommandNames)
		os.Exit(1)
	}
}
