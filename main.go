package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: slb edit | slb render")
		os.Exit(2)
	}
	switch os.Args[1] {
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
	default:
		fmt.Fprintln(os.Stderr, "usage: slb edit | slb render")
		os.Exit(2)
	}
}
