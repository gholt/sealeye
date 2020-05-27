package main

type commonOptions struct {
	CommonOne string `option:"one" help:"First common option."`
	CommonTwo bool   `option:"two" help:"Second common option."`
}
