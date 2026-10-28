package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at release time: go build -ldflags "-X main.version=v0.1.0".
var version = "dev"

const usage = `slb, a status line builder for Claude Code

  slb          open the builder (same as slb edit)
  slb edit     open the builder
  slb render   print the status line from the session JSON on stdin
               (the command Claude Code runs)
  slb version  print the version`

func main() {
	cmd := "edit"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "render":
		// Never crash or exit with an error: Claude Code would show a blank line.
		defer func() { recover() }()
		in, _ := io.ReadAll(os.Stdin)
		cfg, _ := loadConfig()
		fmt.Print(render(in, cfg))
	case "edit":
		if err := edit(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Println("slb", version)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}
