package main

type commonOptions struct {
	CommonOne string `option:"one" help:"First common option."`
	CommonTwo bool   `option:"two" help:"Second common option."`
}

type commonStruct struct {
	Help string
	// QuickHelp should be a quick sentence or two that will be displayed in
	// the parent's list of subcommands.
	QuickHelp  string
	Func       func(cli interface{}) int
	Args       []string
	HelpOption bool `option:"?,h,help" help:"Outputs this help text."`
	// Parent will be set to the parent's command struct value, so you can
	// reference global options, for example. You can omit this field if you
	// won't be needing it.
	Parent interface{}
}
