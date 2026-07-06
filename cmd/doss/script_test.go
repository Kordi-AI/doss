package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestScripts(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "doss")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build doss test binary: %v\n%s", err, out)
	}

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("DOSS", exe)
			return nil
		},
		RequireExplicitExec: true,
	})
}
