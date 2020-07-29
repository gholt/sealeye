package sealeye_test

import (
	"os"
	"testing"

	"github.com/gholt/sealeye"
)

type testIntPtrOptionBugCLI struct {
	Int  *int `option:"int" help:"Tests bug with pointer to int; was panicking with int/int64 mismatch."`
	Func func(*testIntPtrOptionBugCLI) int
	Args []string
}

func TestIntPtrOptionBug(t *testing.T) {
	called := false
	testIntPtrOptionBug := &testIntPtrOptionBugCLI{Func: func(cli *testIntPtrOptionBugCLI) int {
		if cli.Int == nil || *cli.Int != 1 {
			t.Fatal(cli.Int)
		}
		called = true
		return 0
	}}
	if exitCode := sealeye.RunAdvanced(os.Stdout, os.Stderr, t.Name(), testIntPtrOptionBug, []string{"--int", "1"}); exitCode != 0 {
		t.Fatal(exitCode)
	}
	if !called {
		t.Fatal(called)
	}
}
