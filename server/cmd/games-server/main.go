package main

import (
	"fmt"
	"os"
)

const (
	exitFailure = 1
	exitUsage   = 2
)

const usage = `usage: games-server <command> [flags]

commands:
  serve    run the game server
  adduser  create a user account
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(exitUsage)
	}
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(args)
	case "adduser":
		err = addUser(args, os.Stdin)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(exitUsage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "games-server:", err)
		os.Exit(exitFailure)
	}
}
