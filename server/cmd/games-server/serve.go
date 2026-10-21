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
	"syscall"

	"github.com/shivamshivanshu/games.nvim/server/games"
	"github.com/shivamshivanshu/games.nvim/server/internal/auth"
	"github.com/shivamshivanshu/games.nvim/server/internal/clock"
	"github.com/shivamshivanshu/games.nvim/server/internal/hub"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

func serve(args []string) error {
	lim := limits.Default()
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:7777", "listen address")
	dbPath := fs.String("db", "games.db", "sqlite database path")
	fs.DurationVar(&lim.Heartbeat, "heartbeat", lim.Heartbeat, "heartbeat interval")
	fs.DurationVar(&lim.IdleTimeout, "idle", lim.IdleTimeout, "idle timeout")
	if err := fs.Parse(args); err != nil {
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
		Store:   st,
		Games:   games.All(),
		Limits:  lim,
		Time:    clock.System{},
		Lockout: auth.NewLockout(lim, clock.System{}),
	})
	if err != nil {
		return fmt.Errorf("start hub: %w", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	fmt.Printf("listening on %s\n", ln.Addr())
	slog.Info("serving", "addr", ln.Addr().String(), "db", *dbPath)

	err = session.Serve(ctx, ln, lim, h.Login(), h.Routes(), h.Hooks())
	slog.Info("shutting down")
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
