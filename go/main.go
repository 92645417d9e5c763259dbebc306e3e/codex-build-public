package main

import (
	"os"

	"codex-build/internal/runtime"
)

func main() {
	os.Exit(runtime.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
