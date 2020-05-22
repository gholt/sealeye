package main

import (
	"fmt"

	"github.com/gholt/sealeye"
)

func main() {
	// This should be all you need for your main function.
	sealeye.Run(root)
}

// rootCLI is the defining struct for the command; see below where the "var
// root" is set for an actual instance.
type rootCLI struct {
	// Help is what will output at the top of the help output, e.g.
	// "sealeye-example --help". Any instances of {{.Command}} will be replaced
	// with the name of the calling executable, e.g. "sealeye-example" or, if a
	// subcommand, the executable plus the subcommand name, e.g.
	// "sealeye-example cat".
	Help string

	// Func is called when all command line parsing succeeds and the command
	// should actually be run. The integer returned should follow the Unix
	// convention, 0 for "good", "1" for help text requested, and anything else
	// for "bad" or "other". Of course, you can always os.Exit(n) yourself too.
	Func func(cli *rootCLI) int

	// Args is where the remaining arguments from the command line will reside.
	Args []string

	// Now we list the options available. Each option has an "option" tag,
	// which can be one or more comma separated option names, and a "help" tag.
	//
	// Single character options are considered "short options" and use a single
	// "-" prefix, e.g. -h for the standard help option. Multiple character
	// options are considered "long options" and use a double "--" prefix, e.g.
	// --help for the standard help option.

	// HelpOption should usually be included as users always need help. If you
	// include this option, it will automatically be handled by sealeye.
	HelpOption bool `option:"?,h,help" help:"Outputs this help text."`

	// Color should usually be included so users can toggle color output if
	// needed. Sealeye tries to guess what the user would want, but the option
	// helps. Note the the option's default value is defined as "terminal" --
	// this tells sealeye to default to true if it detects stdout is a
	// terminal.
	//
	// Also note that any boolean long option can be prefixed with "no-" to use
	// the false value, e.g. --color for true, and --no-color for false. There
	// is no flipside for short options.
	Color bool `option:"color" help:"Controls color output; use --no-color to disable." default:"terminal"`

	// Version is the first non-sealeye option, which we will handle inside our
	// Func ourselves.
	Version bool `option:"V,version" help:"Output version information."`

	// Quick example showing an environment variable default, "env:DEBUG",
	// which means that if DEBUG=true in the operating system environment, this
	// option will default to true as well.
	Debug bool `option:"v,debug" help:"Output debug information." default:"env:DEBUG"`

	// And lastly we can have a list of subcommands available. Usually you add
	// a subcommand in a separate file inside an init() function; see cat.go
	// and version.go for examples.
	Subcommands map[string]interface{}
}

var root = &rootCLI{
	Help: `
Usage: {{.Command}} [options] subcommand [subcommand] ...

This example program offers two simple subcommands, "cat" and "version". It is just to show the feature set of sealeye, a cli library for Go.

Note that in this help text, {{"{{.Command}}"}} will be replaced with the name of the calling executable, so you can easily provide several examples and not worry about what the name your program was compiled as, or whether it was a subcommand of another subcommand, etc.

This help text is also run through a Markdown processor so that it will rewrap text to fit the terminal, *colorize output* where it seems appropriate, and even support simple tables:

| Heading One | Heading Two |
| ---: | --- |
| Blah | Yadda yadda |
| Crazy Is | What crazy does |
| Test Link | https://example.com/ |

Note that in a multiline Go string, it can be a bit cumbersome to specify ` + "`" + `code snippets` + "`" + ` and code blocks:

` + "```" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello World!")
}
` + "```" + `

Also note that with the options, their usage text does **not** support Markdown as they are already being output in a specific format.
`,
	Func: func(cli *rootCLI) int {
		if cli.Version {
			fmt.Println("Version 1.2.3")
			return 0
		}
		if cli.Debug {
			fmt.Println("No subcommands were given; outputing help text.")
		}
		return 1
	},
	Subcommands: map[string]interface{}{},
}
