package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/auth"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

var errUsage = errors.New("usage: games-server adduser --db <path> [--password-stdin] <name>")

func addUser(args []string, stdin io.Reader) error {
	fs := flag.NewFlagSet("adduser", flag.ContinueOnError)
	dbPath := fs.String("db", "games.db", "sqlite database path")
	fromStdin := fs.Bool("password-stdin", false, "read the password from one line of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errUsage
	}
	name := game.UserID(fs.Arg(0))

	ctx := context.Background()
	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	var password string
	if *fromStdin {
		password, err = readPasswordLine(stdin)
	} else {
		password, err = promptPassword()
	}
	if err != nil {
		return err
	}
	if err := auth.CreateAccount(ctx, st, name, password, auth.DefaultPasswordCost); err != nil {
		return fmt.Errorf("adduser %s: %w", name, err)
	}
	return nil
}

func readPasswordLine(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func promptPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Password: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(raw), err
}
