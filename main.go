package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at release time: go build -ldflags "-X main.version=v0.1.0".
var version = "dev"

const usage = `dgsl (dg-statuslines), a status line builder for Claude Code

  dgsl          open the builder (same as dgsl edit)
  dgsl edit     open the builder
  dgsl render   print the status line from the session JSON on stdin
               (the command Claude Code runs)
  dgsl version  print the version`

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
		fmt.Println("dgsl", version)
	case "help", "-h", "--help":
		fmt.Println(usage)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}
