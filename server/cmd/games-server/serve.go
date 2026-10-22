package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/shivamshivanshu/games.nvim/server/games"
	"github.com/shivamshivanshu/games.nvim/server/internal/auth"
	"github.com/shivamshivanshu/games.nvim/server/internal/clock"
	"github.com/shivamshivanshu/games.nvim/server/internal/hub"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

const signupPassphraseEnv = "GAMES_SIGNUP_PASSPHRASE"

func serve(args []string) error {
	lim := limits.Default()
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:7777", "listen address")
	dbPath := fs.String("db", "games.db", "sqlite database path")
	passphraseFile := fs.String("signup-passphrase-file", "", "file holding the sign-up passphrase (default $"+signupPassphraseEnv+"; sign-up is off without either)")
	fs.DurationVar(&lim.Heartbeat, "heartbeat", lim.Heartbeat, "heartbeat interval")
	fs.DurationVar(&lim.IdleTimeout, "idle", lim.IdleTimeout, "idle timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	passphrase, err := signupPassphrase(*passphraseFile, os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	h, err := hub.New(ctx, hub.Deps{
		Store:            st,
		Games:            games.All(),
		Limits:           lim,
		Time:             clock.System{},
		Lockout:          auth.NewLockout(lim, clock.System{}),
		SignupPassphrase: passphrase,
	})
	if err != nil {
		return fmt.Errorf("start hub: %w", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	fmt.Printf("listening on %s\n", ln.Addr())
	slog.Info("serving", "addr", ln.Addr().String(), "db", *dbPath, "signup", passphrase != "")

	err = session.Serve(ctx, ln, lim, h.Authenticator(), h.Routes(), h.Hooks())
	slog.Info("shutting down")
	stop()
	h.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// signupPassphrase reads the passphrase from path, or from the environment
// when path is empty; "" means sign-up is off. It is never a flag value,
// which any user could read from the process list.
func signupPassphrase(path string, getenv func(string) string) (string, error) {
	if path == "" {
		return getenv(signupPassphraseEnv), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read sign-up passphrase: %w", err)
	}
	passphrase := strings.TrimRight(string(raw), "\r\n")
	if passphrase == "" {
		return "", fmt.Errorf("sign-up passphrase file %s is empty", path)
	}
	return passphrase, nil
}
