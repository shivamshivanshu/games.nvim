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
	"sync"
	"testing"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/limits"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/room"
	"games.nvim/server/internal/session"
	"games.nvim/server/internal/store"
)

const (
	alice        game.UserID          = "alice"
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
		return protocol.LoginResult{}, fmt.Errorf("%w: wrong password", session.ErrAuthFailed)
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
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if routes == nil {
		routes = session.Router{typeEcho: session.Handle(echo)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- session.Serve(ctx, ln, lim, stubLogin, routes, hooks) }()
	shutdown := sync.OnceValue(func() error {
		cancel()
		select {
		case err := <-served:
			return err
		case <-time.After(replyTimeout):
			return errors.New("Serve did not return after cancel")
		}
	})
	t.Cleanup(func() {
		if err := shutdown(); err != nil {
			t.Error(err)
		}
	})
	return server{addr: ln.Addr().String(), shutdown: shutdown}
}

type client struct {
	t      *testing.T
	conn   net.Conn
	reader *protocol.Reader
	nextID uint64
}

func dial(t *testing.T, addr string) *client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &client{t: t, conn: conn, reader: protocol.NewReader(conn, limits.MaxLineBytes)}
}

func (c *client) sendLine(line string) {
	c.t.Helper()
	if _, err := io.WriteString(c.conn, line+"\n"); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) send(typ protocol.MessageType, body any) uint64 {
	c.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		c.t.Fatal(err)
	}
	c.nextID++
	id := c.nextID
	if err := protocol.WriteEnvelope(c.conn, protocol.Envelope{ID: &id, Type: typ, Body: raw}); err != nil {
		c.t.Fatal(err)
	}
	return id
}

func (c *client) next() protocol.Envelope {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(replyTimeout))
	env, err := c.reader.Next()
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	return env
}

func (c *client) request(typ protocol.MessageType, body any) protocol.Envelope {
	c.t.Helper()
	id := c.send(typ, body)
	env := c.next()
	if env.ID == nil || *env.ID != id {
		c.t.Fatalf("reply %+v does not answer request %d", env, id)
	}
	return env
}

func (c *client) login() {
	c.t.Helper()
	env := c.request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: alice, Password: testPassword})
	if env.Type != protocol.TypeOK {
		c.t.Fatalf("login reply = %s %s", env.Type, env.Body)
	}
}

func (c *client) expectClosedWithin(d time.Duration) {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(d))
	if env, err := c.reader.Next(); !errors.Is(err, io.EOF) {
		c.t.Fatalf("want EOF within %v, got %+v, %v", d, env, err)
	}
}

func expectError(t *testing.T, env protocol.Envelope, want protocol.ErrorCode) protocol.ErrorBody {
	t.Helper()
	var body protocol.ErrorBody
	if env.Type != protocol.TypeError || json.Unmarshal(env.Body, &body) != nil || body.Code != want {
		t.Fatalf("want %s error, got %s %s", want, env.Type, env.Body)
	}
	return body
}

func TestLoginDeadlineClosesConnection(t *testing.T) {
	lim := testLimits()
	lim.LoginDeadline = 100 * time.Millisecond
	srv := startServer(t, lim, nil, session.Hooks{})

	dial(t, srv.addr).expectClosedWithin(300 * time.Millisecond)
}

func TestRequestBeforeLoginUnauthenticated(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := dial(t, srv.addr)

	expectError(t, c.request(typeEcho, echoReq{Text: "hi"}), protocol.CodeUnauthenticated)
	expectError(t, c.request(protocol.TypePing, protocol.Empty{}), protocol.CodeUnauthenticated)
	c.login()
}

func TestLoginFailuresKeepConnectionUnauthenticated(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := dial(t, srv.addr)

	expectError(t, c.request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version + 1, Username: alice, Password: testPassword}), protocol.CodeVersionMismatch)
	expectError(t, c.request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: alice, Password: "wrong"}), protocol.CodeAuthFailed)
	expectError(t, c.request(typeEcho, echoReq{}), protocol.CodeUnauthenticated)
	c.login()
	expectError(t, c.request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: alice, Password: testPassword}), protocol.CodeBadRequest)
}

func TestIdleTimeoutClosesAfterLogin(t *testing.T) {
	lim := testLimits()
	lim.IdleTimeout = 200 * time.Millisecond
	srv := startServer(t, lim, nil, session.Hooks{})

	t.Run("no pings", func(t *testing.T) {
		c := dial(t, srv.addr)
		c.login()
		c.expectClosedWithin(600 * time.Millisecond)
	})
	t.Run("pings every 50ms", func(t *testing.T) {
		c := dial(t, srv.addr)
		c.login()
		for range 8 {
			time.Sleep(50 * time.Millisecond)
			if env := c.request(protocol.TypePing, protocol.Empty{}); env.Type != protocol.TypeOK || string(env.Body) != "{}" {
				t.Fatalf("ping reply = %s %s", env.Type, env.Body)
			}
		}
	})
}

func TestConnectionCap(t *testing.T) {
	lim := testLimits()
	lim.MaxConnections = 1
	srv := startServer(t, lim, nil, session.Hooks{})
	dial(t, srv.addr).login()

	second := dial(t, srv.addr)
	env := second.next()
	if env.ID != nil {
		t.Fatalf("rejection carries id %d", *env.ID)
	}
	if body := expectError(t, env, protocol.CodeInternal); body.Message != "server full" {
		t.Fatalf("message = %q", body.Message)
	}
	second.expectClosedWithin(replyTimeout)
}

func TestHandleDecodesTypedBody(t *testing.T) {
	srv := startServer(t, testLimits(), nil, session.Hooks{})
	c := dial(t, srv.addr)
	c.login()

	env := c.request(typeEcho, echoReq{Text: "hi"})
	var resp echoResp
	if env.Type != protocol.TypeOK || json.Unmarshal(env.Body, &resp) != nil || resp.Echo != "hi" {
		t.Fatalf("echo reply = %s %s", env.Type, env.Body)
	}
	if env := c.request(typeEcho, nil); env.Type != protocol.TypeOK {
		t.Fatalf("null body reply = %s %s", env.Type, env.Body)
	}
	c.sendLine(`{"id":99,"type":"test.echo","body":{"text":5}}`)
	if env := c.next(); env.ID == nil || *env.ID != 99 {
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
		{fmt.Errorf("wrapped: %w", store.ErrNotFound), protocol.CodeNotFound},
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
	c := dial(t, srv.addr)
	c.login()

	for i, tc := range cases {
		body := expectError(t, c.request(protocol.MessageType(fmt.Sprint("fail.", i)), protocol.Empty{}), tc.want)
		if tc.want == protocol.CodeInternal && strings.Contains(body.Message, tc.err.Error()) {
			t.Errorf("internal error leaked: %q", body.Message)
		}
	}
	expectError(t, c.request("no.such.route", protocol.Empty{}), protocol.CodeBadRequest)
}

func TestLineTooLongRepliesBadRequestAndCloses(t *testing.T) {
	lim := testLimits()
	srv := startServer(t, lim, nil, session.Hooks{})
	c := dial(t, srv.addr)

	if _, err := io.WriteString(c.conn, strings.Repeat("x", lim.MaxLineBytes+1)); err != nil {
		t.Fatal(err)
	}
	env := c.next()
	if env.ID != nil {
		t.Fatalf("line-too-long reply carries id %d", *env.ID)
	}
	expectError(t, env, protocol.CodeBadRequest)
	c.expectClosedWithin(replyTimeout)
}

func TestHooksFireOncePerAuthenticatedSession(t *testing.T) {
	authenticated := make(chan *session.Session, 4)
	closed := make(chan *session.Session, 4)
	srv := startServer(t, testLimits(), nil, session.Hooks{
		OnAuthenticated: func(s *session.Session) { authenticated <- s },
		OnClosed:        func(s *session.Session) { closed <- s },
	})
	dial(t, srv.addr)
	c := dial(t, srv.addr)
	c.login()

	s := <-authenticated
	if s.User() != alice || !s.RemoteAddr().IsLoopback() {
		t.Fatalf("session user %q addr %v", s.User(), s.RemoteAddr())
	}
	push := protocol.Envelope{Type: protocol.TypeGameUpdate, Body: json.RawMessage(`{}`)}
	if !s.Push(push) {
		t.Fatal("push to live session failed")
	}
	if env := c.next(); env.Type != protocol.TypeGameUpdate || env.ID != nil {
		t.Fatalf("push arrived as %+v", env)
	}
	c.conn.Close()
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
	c := dial(t, srv.addr)
	c.login()

	if err := srv.shutdown(); err != nil {
		t.Fatalf("Serve returned %v", err)
	}
	c.expectClosedWithin(replyTimeout)
}
