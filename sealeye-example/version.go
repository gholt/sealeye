package main

import "fmt"

func init() {
	root.Subcommands["version"] = version
}

type versionCLI struct {
	// sprinkles shows how to have a reusable set of options; see sprinkles.go
	// and also cat.go where sprinkles are also embedded.
	sprinkles
	Help       string
	QuickHelp  string
	Func       func(cli *versionCLI) int
	Args       []string
	HelpOption bool `option:"?,h,help" help:"Outputs this help text."`
	// Silly example, but shows that subcommands can have subcommands; see
	// versionOnly.go for the sub-subcommand.
	Subcommands map[string]interface{}
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
	Subcommands: map[string]interface{}{},
}
