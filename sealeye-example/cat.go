package main

import (
	"fmt"
	"io"
	"os"
)

var cat = &catCLI{}

func init() {
	root.Subcommands["cat"] = cat
	cat.Help = `
Usage: {{.Command}} [options] filename [filename] ...

This example program will just output the content of the filename or filenames.
`
	cat.QuickHelp = "Output the content of a file or files."
	cat.Func = func(cliI interface{}) int {
		cli := cliI.(*catCLI)
		// This is here because we overrode the embedded sprinkles option, but
		// we still want to use it's reusable method, sprinkle().
		cli.sprinkleOptions.SprinkleType = cli.SprinkleType
		if len(cli.Args) == 0 {
			return 1
		}
		cli.sprinkle()
		if cli.Parent.(*rootCLI).Debug {
			fmt.Printf("We have %d files to output\n", len(cli.Args))
		}
		for _, arg := range cli.Args {
			if cli.Filenames {
				fmt.Print(cli.Prefix)
				fmt.Println(arg)
			}
			for i := 0; i < cli.Count; i++ {
				f, err := os.Open(arg)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 2
				}
				if _, err := io.Copy(os.Stdout, f); err != nil {
					_ = f.Close()
					fmt.Fprintln(os.Stderr, err)
					return 2
				}
				if err := f.Close(); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 2
				}
			}
		}
		cli.sprinkle()
		return 0
	}
}

type catCLI struct {
	// Embedded structs for reuseable option definitions; defined in common.go
	// and sprinkles.go -- this shows how you can embed multiple structs with
	// no problem.
	//
	// Also note that down below we will override one of the sprinkle options,
	// SprinkleType, with example code way below on how to work between the two
	// levels.
	commonStruct
	commonOptions
	sprinkleOptions

	Filenames bool `option:"f,filenames" help:"Outputs filenames before each file."`
	// Prefix just shows a string option type with a default.
	Prefix string `option:"p,prefix" help:"Prefix to output before each filename, if any." default:"## "`
	// Count just shows that you can have multiple defaults. This will use the
	// option's value if the user set it, or the COUNT environment variable if
	// that was set, or finally the plain value of 1 if all else fails.
	Count int `option:"c,count" help:"The number of times to output each file." default:"env:COUNT,1"`
	// SprinkleType is here to show how you can override embedded options; this
	// is overriding what is in sprinkles at the top of this struct.
	SprinkleType int `option:"sprinkle-type" help:"The type of sprinkles to output (overridden)." default:"1"`
}
