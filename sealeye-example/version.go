package main

import "fmt"

func init() {
	root.Subcommands["version"] = version
}

type versionCLI struct {
	// Embedded structs for reuseable option definitions; defined in common.go
	// and sprinkles.go -- this shows how you can embed multiple structs with
	// no problem.
	commonOptions
	sprinkleOptions

	Help          string
	QuickHelp     string
	Func          func(cli *versionCLI) int
	Args          []string
	HelpOption    bool `option:"?,h,help" help:"Outputs this help text."`
	AllHelpOption bool `option:"all-help" help:"Outputs this help text and the help text for all subcommands."`
	// Silly example, but shows that subcommands can have subcommands; see
	// versionOnly.go for the sub-subcommand.
	Subcommands map[string]interface{}
	// Also, subcommands can be hidden from help text but still available like
	// any other subcommand. This can be useful when deprecating an old
	// subcommand, but leaving it around for backward compatibility.
	HiddenSubcommands map[string]interface{}
}

var version = &versionCLI{
	Help: `
Usage: {{.Command}}

Outputs the program's version.
`,
	QuickHelp: "Output the version of the program.",
	Func: func(cli *versionCLI) int {
		if len(cli.Args) > 0 {
			return 1
		}
		fmt.Println("Version 1.2.3")
		return 0
	},
	Subcommands:       map[string]interface{}{},
	HiddenSubcommands: map[string]interface{}{},
}
