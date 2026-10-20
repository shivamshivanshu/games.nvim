package session

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const serverFullMessage = "server full"

func Serve(ctx context.Context, ln net.Listener, lim limits.Limits, login Authenticator, routes Router, hooks Hooks) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopClosingListener := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stopClosingListener()

	cfg := &serveConfig{lim: lim, login: login, routes: routes, hooks: hooks}
	slots := make(chan struct{}, lim.MaxConnections)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("session: accept: %w", err)
		}
		select {
		case slots <- struct{}{}:
			wg.Go(func() {
				defer func() { <-slots }()
				serveConn(ctx, conn, cfg)
			})
		default:
			rejectServerFull(conn)
		}
	}
}

func serveConn(ctx context.Context, conn net.Conn, cfg *serveConfig) {
	sessionCtx, cancel := context.WithCancel(ctx)
	s := newSession(conn, cfg, cancel)
	stopClosingOnShutdown := context.AfterFunc(ctx, s.close)
	defer stopClosingOnShutdown()
	s.run(sessionCtx)
}

func rejectServerFull(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(closeFlushTimeout))
	_, _ = conn.Write(errorFrame(nil, protocol.ErrorBody{Code: protocol.CodeInternal, Message: serverFullMessage}))
}

func remoteAddr(conn net.Conn) netip.Addr {
	addrPort, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	return addrPort.Addr().Unmap()
}
