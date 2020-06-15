package main

import "fmt"

func init() {
	version.HiddenSubcommands["hidden"] = versionHidden
}

type versionHiddenCLI struct {
	commonOptions
	Help       string
	QuickHelp  string
	Func       func(cli *versionHiddenCLI) int
	Args       []string
	HelpOption bool `option:"?,h,help" help:"Outputs this help text."`
}

var versionHidden = &versionHiddenCLI{
	Help: `
Usage: {{.Command}}

Mostly just an example of a hidden subcommand.
`,
	QuickHelp: "Mostly just an example of a hidden subcommand.",
	Func: func(cli *versionHiddenCLI) int {
		if len(cli.Args) > 0 {
			return 1
		}
		fmt.Println("1.2.3")
		return 0
	},
}
