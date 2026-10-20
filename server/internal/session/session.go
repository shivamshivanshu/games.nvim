package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/limits"
	"games.nvim/server/internal/protocol"
)

const closeFlushTimeout = time.Second

type Authenticator func(ctx context.Context, s *Session, req protocol.LoginRequest) (protocol.LoginResult, error)

type Hooks struct {
	OnAuthenticated func(*Session)
	OnClosed        func(*Session)
}

type serveConfig struct {
	lim    limits.Limits
	login  Authenticator
	routes Router
	hooks  Hooks
}

type Session struct {
	conn   net.Conn
	remote netip.Addr
	cfg    *serveConfig
	cancel context.CancelFunc
	out    chan protocol.Envelope
	done   chan struct{}
	user   game.UserID

	mu     sync.Mutex
	closed bool
}

func newSession(conn net.Conn, cfg *serveConfig, cancel context.CancelFunc) *Session {
	return &Session{
		conn:   conn,
		remote: remoteAddr(conn),
		cfg:    cfg,
		cancel: cancel,
		out:    make(chan protocol.Envelope, cfg.lim.WriteQueueFrames),
		done:   make(chan struct{}),
	}
}

func (s *Session) User() game.UserID { return s.user }

func (s *Session) RemoteAddr() netip.Addr { return s.remote }

func (s *Session) Push(env protocol.Envelope) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.out <- env:
		return true
	default:
		s.closeLocked()
		return false
	}
}

func (s *Session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
}

func (s *Session) closeLocked() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.done)
	s.cancel()
	now := time.Now()
	_ = s.conn.SetReadDeadline(now)
	_ = s.conn.SetWriteDeadline(now.Add(closeFlushTimeout))
}

func (s *Session) extendReadDeadline(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		_ = s.conn.SetReadDeadline(time.Now().Add(d))
	}
}

func (s *Session) authenticated() bool { return s.user != "" }

func (s *Session) run(ctx context.Context) {
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		s.writeLoop()
	}()
	s.readLoop(ctx)
	s.close()
	<-writerDone
	if s.authenticated() && s.cfg.hooks.OnClosed != nil {
		s.cfg.hooks.OnClosed(s)
	}
}

func (s *Session) readLoop(ctx context.Context) {
	reader := protocol.NewReader(s.conn, s.cfg.lim.MaxLineBytes)
	s.extendReadDeadline(s.cfg.lim.LoginDeadline)
	for {
		env, err := reader.Next()
		switch {
		case errors.Is(err, protocol.ErrLineTooLong):
			s.Push(errorEnvelope(nil, protocol.ErrorBody{Code: protocol.CodeBadRequest, Message: err.Error()}))
			return
		case errors.Is(err, protocol.ErrMalformedEnvelope):
			s.Push(errorEnvelope(nil, protocol.ErrorBody{Code: protocol.CodeBadRequest, Message: err.Error()}))
		case err != nil:
			return
		default:
			result, err := s.dispatch(ctx, env)
			s.Push(replyEnvelope(env.ID, result, err))
		}
		if s.authenticated() {
			s.extendReadDeadline(s.cfg.lim.IdleTimeout)
		}
	}
}

func (s *Session) dispatch(ctx context.Context, env protocol.Envelope) (any, error) {
	switch {
	case env.Type == protocol.TypeAuthLogin:
		return Handle(login)(ctx, s, env.Body)
	case !s.authenticated():
		return nil, errUnauthenticated
	case env.Type == protocol.TypePing:
		return protocol.Empty{}, nil
	}
	handler, ok := s.cfg.routes[env.Type]
	if !ok {
		return nil, protocol.BadRequestf("unknown message type %q", env.Type)
	}
	return handler(ctx, s, env.Body)
}

func login(ctx context.Context, s *Session, req protocol.LoginRequest) (protocol.LoginResult, error) {
	if s.authenticated() {
		return protocol.LoginResult{}, protocol.BadRequestf("already logged in")
	}
	if req.V != protocol.Version {
		return protocol.LoginResult{}, protocol.Error{
			Code: protocol.CodeVersionMismatch,
			Msg:  fmt.Sprintf("protocol version mismatch: client v%d, server v%d", req.V, protocol.Version),
		}
	}
	res, err := s.cfg.login(ctx, s, req)
	if err != nil {
		return protocol.LoginResult{}, err
	}
	if res.User == "" {
		return protocol.LoginResult{}, errors.New("session: authenticator accepted login without a user")
	}
	s.user = res.User
	res.HeartbeatMS = s.cfg.lim.Heartbeat.Milliseconds()
	if s.cfg.hooks.OnAuthenticated != nil {
		s.cfg.hooks.OnAuthenticated(s)
	}
	return res, nil
}

func (s *Session) writeLoop() {
	defer s.conn.Close()
	for {
		select {
		case env := <-s.out:
			if err := protocol.WriteEnvelope(s.conn, env); err != nil {
				s.close()
				return
			}
		case <-s.done:
			s.flushQueued()
			return
		}
	}
}

func (s *Session) flushQueued() {
	for {
		select {
		case env := <-s.out:
			if protocol.WriteEnvelope(s.conn, env) != nil {
				return
			}
		default:
			return
		}
	}
}
