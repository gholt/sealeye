package main

import "fmt"

func init() {
	version.Subcommands["only"] = versionOnly
}

type versionOnlyCLI struct {
	commonOptions
	Help       string
	QuickHelp  string
	Func       func(cli *versionOnlyCLI) int
	Args       []string
	HelpOption bool `option:"?,h,help" help:"Outputs this help text."`
}

var versionOnly = &versionOnlyCLI{
	Help: `
Usage: {{.Command}}

Outputs the program's version number, and only the version number.
`,
	QuickHelp: "Output the version number of the program, and only the version number.",
	Func: func(cli *versionOnlyCLI) int {
		if len(cli.Args) > 0 {
			return 1
		}
		fmt.Println("1.2.3")
		return 0
	},
}
