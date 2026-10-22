package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/room"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
)

const (
	testPassword                      = "secret"
	typeEcho     protocol.MessageType = "test.echo"
	replyTimeout                      = 2 * time.Second
)

type echoReq struct {
	Text string `json:"text"`
}

type echoResp struct {
	Echo string `json:"echo"`
}

func echo(_ context.Context, _ *session.Session, req echoReq) (echoResp, error) {
	return echoResp{Echo: req.Text}, nil
}

func stubLogin(_ context.Context, _ *session.Session, req protocol.LoginRequest) (protocol.LoginResult, error) {
	if req.Password != testPassword {
		return protocol.LoginResult{}, protocol.Error{Code: protocol.CodeAuthFailed, Msg: "authentication failed: wrong password"}
	}
	return protocol.LoginResult{User: req.Username}, nil
}

func testLimits() limits.Limits {
	lim := limits.Default()
	lim.LoginDeadline = replyTimeout
	lim.IdleTimeout = replyTimeout
	lim.MaxLineBytes = 256
	return lim
}

type server struct {
	addr     string
	shutdown func() error
}

func startServer(t *testing.T, lim limits.Limits, routes session.Router, hooks session.Hooks) server {
	t.Helper()
	if routes == nil {
		routes = session.Router{typeEcho: session.Handle(echo)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	addr, shutdown := prototest.Serve(t, cancel, replyTimeout, func(ln net.Listener) error {
		return session.Serve(ctx, ln, lim, session.Authenticator{Login: stubLogin}, routes, hooks)
	})
	t.Cleanup(func() {
		if err := shutdown(); err != nil {
			t.Error(err)
		}
	})
	return server{addr: addr, shutdown: shutdown}
}

func login(t *testing.T, c *prototest.Client) {
	t.Helper()
	if _, err := c.Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: prototest.Alice, Password: testPassword}); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func next(t *testing.T, c *prototest.Client) protocol.Envelope {
	t.Helper()
	env, err := c.Next(replyTimeout)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return env
}

func expectClosedWithin(t *testing.T, c *prototest.Client, d time.Duration) {
	t.Helper()
	if line, err := c.ReadLine(d); !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF within %v, got %s, %v", d, line, err)
	}
}

func expectError(t *testing.T, env protocol.Envelope, want protocol.ErrorCode) string {
	t.Helper()
	_, err := prototest.ReplyOf(env)
	return prototest.WantCode(t, err, want)
}

func TestLoginDeadlineClosesConnection(t *testing.T) {
	lim := testLimits()
	lim.LoginDeadline = 100 * time.Millisecond
	srv := startServer(t, lim, nil, session.Hooks{})

	expectClosedWithin(t, prototest.Dial(t, srv.addr), 300*time.Millisecond)
}

func TestRequestBeforeLoginUnauthenticated(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := prototest.Dial(t, srv.addr)

	_, err := c.Request(typeEcho, echoReq{Text: "hi"})
	prototest.WantCode(t, err, protocol.CodeUnauthenticated)
	_, err = c.Request(protocol.TypePing, protocol.Empty{})
	prototest.WantCode(t, err, protocol.CodeUnauthenticated)
	login(t, c)
}

func TestLoginFailuresKeepConnectionUnauthenticated(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := prototest.Dial(t, srv.addr)

	_, err := c.Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version + 1, Username: prototest.Alice, Password: testPassword})
	prototest.WantCode(t, err, protocol.CodeVersionMismatch)
	_, err = c.Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: prototest.Alice, Password: "wrong"})
	prototest.WantCode(t, err, protocol.CodeAuthFailed)
	_, err = c.Request(typeEcho, echoReq{})
	prototest.WantCode(t, err, protocol.CodeUnauthenticated)
	login(t, c)
	_, err = c.Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: prototest.Alice, Password: testPassword})
	prototest.WantCode(t, err, protocol.CodeBadRequest)
}

func TestIdleTimeoutClosesAfterLogin(t *testing.T) {
	lim := testLimits()
	lim.IdleTimeout = 200 * time.Millisecond
	srv := startServer(t, lim, nil, session.Hooks{})

	t.Run("no pings", func(t *testing.T) {
		c := prototest.Dial(t, srv.addr)
		login(t, c)
		expectClosedWithin(t, c, 600*time.Millisecond)
	})
	t.Run("pings every 50ms", func(t *testing.T) {
		c := prototest.Dial(t, srv.addr)
		login(t, c)
		for range 8 {
			time.Sleep(50 * time.Millisecond)
			if body, err := c.Request(protocol.TypePing, protocol.Empty{}); err != nil || string(body) != "{}" {
				t.Fatalf("ping reply = %s, %v", body, err)
			}
		}
	})
}

func TestConnectionCap(t *testing.T) {
	lim := testLimits()
	lim.MaxConnections = 1
	srv := startServer(t, lim, nil, session.Hooks{})
	login(t, prototest.Dial(t, srv.addr))

	second := prototest.Dial(t, srv.addr)
	env := next(t, second)
	if env.ID != nil {
		t.Fatalf("rejection carries id %d", *env.ID)
	}
	if msg := expectError(t, env, protocol.CodeInternal); msg != "server full" {
		t.Fatalf("message = %q", msg)
	}
	expectClosedWithin(t, second, replyTimeout)
}

func TestHandleDecodesTypedBody(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := prototest.Dial(t, srv.addr)
	login(t, c)

	body, err := c.Request(typeEcho, echoReq{Text: "hi"})
	var resp echoResp
	if err != nil || json.Unmarshal(body, &resp) != nil || resp.Echo != "hi" {
		t.Fatalf("echo reply = %s, %v", body, err)
	}
	if body, err := c.Request(typeEcho, nil); err != nil {
		t.Fatalf("null body reply = %s, %v", body, err)
	}
	if err := c.WriteLine([]byte(`{"id":99,"type":"test.echo","body":{"text":5}}`)); err != nil {
		t.Fatal(err)
	}
	if env := next(t, c); env.ID == nil || *env.ID != 99 {
		t.Fatalf("malformed body reply = %+v", env)
	} else {
		expectError(t, env, protocol.CodeBadRequest)
	}
}

func TestErrorsMapToCodes(t *testing.T) {
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	t.Cleanup(func() { slog.SetDefault(previous) })
	cases := []struct {
		err  error
		want protocol.ErrorCode
	}{
		{game.BadRequestError{Msg: "bad"}, protocol.CodeBadRequest},
		{fmt.Errorf("wrapped: %w", game.IllegalActionError{Msg: "illegal"}), protocol.CodeIllegalAction},
		{fmt.Errorf("wrapped: %w", game.ErrNotYourTurn), protocol.CodeNotYourTurn},
		{fmt.Errorf("wrapped: %w", room.ErrStale), protocol.CodeStale},
		{fmt.Errorf("wrapped: %w", protocol.Error{Code: protocol.CodeNotFound, Msg: "gone"}), protocol.CodeNotFound},
		{room.ErrNotAPlayer, protocol.CodeNotFound},
		{context.Canceled, protocol.CodeInternal},
		{errors.New("db exploded at /var/lib/games.db"), protocol.CodeInternal},
	}
	routes := session.Router{}
	for i, tc := range cases {
		routes[protocol.MessageType(fmt.Sprint("fail.", i))] = session.Handle(func(context.Context, *session.Session, protocol.Empty) (protocol.Empty, error) {
			return protocol.Empty{}, tc.err
		})
	}
	srv := startServer(t, testLimits(), routes, session.Hooks{})
	c := prototest.Dial(t, srv.addr)
	login(t, c)

	for i, tc := range cases {
		_, err := c.Request(protocol.MessageType(fmt.Sprint("fail.", i)), protocol.Empty{})
		if msg := prototest.WantCode(t, err, tc.want); tc.want == protocol.CodeInternal && strings.Contains(msg, tc.err.Error()) {
			t.Errorf("internal error leaked: %q", msg)
		}
	}
	_, err := c.Request("no.such.route", protocol.Empty{})
	prototest.WantCode(t, err, protocol.CodeBadRequest)
}

func TestLineTooLongRepliesBadRequestAndCloses(t *testing.T) {
	lim := testLimits()
	srv := startServer(t, lim, nil, session.Hooks{})
	c := prototest.Dial(t, srv.addr)

	if _, err := io.WriteString(c, strings.Repeat("x", lim.MaxLineBytes+1)); err != nil {
		t.Fatal(err)
	}
	env := next(t, c)
	if env.ID != nil {
		t.Fatalf("line-too-long reply carries id %d", *env.ID)
	}
	expectError(t, env, protocol.CodeBadRequest)
	expectClosedWithin(t, c, replyTimeout)
}

func TestHooksFireOncePerAuthenticatedSession(t *testing.T) {
	authenticated := make(chan *session.Session, 4)
	closed := make(chan *session.Session, 4)
	srv := startServer(t, testLimits(), nil, session.Hooks{
		OnAuthenticated: func(s *session.Session) { authenticated <- s },
		OnClosed:        func(s *session.Session) { closed <- s },
	})
	prototest.Dial(t, srv.addr)
	c := prototest.Dial(t, srv.addr)
	login(t, c)

	s := <-authenticated
	if s.User() != prototest.Alice || !s.RemoteAddr().IsLoopback() {
		t.Fatalf("session user %q addr %v", s.User(), s.RemoteAddr())
	}
	push, err := protocol.EncodeFrame(nil, protocol.TypeGameUpdate, protocol.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Push(push) {
		t.Fatal("push to live session failed")
	}
	if env := next(t, c); env.Type != protocol.TypeGameUpdate || env.ID != nil {
		t.Fatalf("push arrived as %+v", env)
	}
	_ = c.Close()
	select {
	case got := <-closed:
		if got != s {
			t.Fatal("OnClosed got a different session")
		}
	case <-time.After(replyTimeout):
		t.Fatal("OnClosed not called")
	}
	if s.Push(push) {
		t.Fatal("push after close succeeded")
	}

	if err := srv.shutdown(); err != nil {
		t.Fatal(err)
	}
	if len(authenticated) != 0 || len(closed) != 0 {
		t.Fatalf("extra hook calls: authenticated %d, closed %d", len(authenticated), len(closed))
	}
}

func TestServeShutdownClosesSessions(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := prototest.Dial(t, srv.addr)
	login(t, c)

	if err := srv.shutdown(); err != nil {
		t.Fatalf("Serve returned %v", err)
	}
	expectClosedWithin(t, c, replyTimeout)
}
