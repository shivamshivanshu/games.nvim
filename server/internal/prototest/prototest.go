// Package prototest is the client end of the wire protocol for tests.
package prototest

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const (
	Alice game.UserID = "alice"
	Bob   game.UserID = "bob"
	Carol game.UserID = "carol"
)

// ReplyTimeout bounds the wait for each line of a reply.
const ReplyTimeout = 5 * time.Second

const lineBacklog = 256

var ErrTimeout = errors.New("prototest: no line in time")

// Client is one test connection. A background reader only queues the lines
// the server sends; the Client itself must be used from one goroutine.
type Client struct {
	conn    net.Conn
	lines   chan []byte
	closed  chan struct{}
	readErr error
	nextID  protocol.RequestID
	backlog []protocol.Envelope
}

// Dial connects to addr, closing the connection on t's cleanup, and reads it
// with the server's default line limit.
func Dial(t testing.TB, addr string) *Client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{conn: conn, lines: make(chan []byte, lineBacklog), closed: make(chan struct{})}
	t.Cleanup(func() {
		close(c.closed)
		_ = conn.Close()
	})
	go c.readLoop()
	return c
}

func (c *Client) readLoop() {
	defer close(c.lines)
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(nil, limits.Default().MaxLineBytes+1)
	for scanner.Scan() {
		select {
		case c.lines <- bytes.Clone(scanner.Bytes()):
		case <-c.closed:
			return
		}
	}
	c.readErr = scanner.Err()
	if errors.Is(c.readErr, bufio.ErrTooLong) {
		c.readErr = protocol.ErrLineTooLong
	}
}

func (c *Client) Close() error { return c.conn.Close() }

// Write sends p as it is, with no newline added.
func (c *Client) Write(p []byte) (int, error) { return c.conn.Write(p) }

// WriteLine sends line as it is, newline-terminated.
func (c *Client) WriteLine(line []byte) error {
	_, err := c.conn.Write(slices.Concat(line, []byte("\n")))
	return err
}

// Send writes a request of type typ carrying body as JSON, under the next
// request id.
func (c *Client) Send(typ protocol.MessageType, body any) (protocol.RequestID, error) {
	c.nextID++
	id := c.nextID
	frame, err := protocol.EncodeFrame(&id, typ, body)
	if err != nil {
		return 0, err
	}
	_, err = c.conn.Write(frame)
	return id, err
}

// ReadLine is the next line the server sent: io.EOF once the connection has
// closed cleanly, the read error once it has failed, and ErrTimeout when no
// line comes within timeout.
func (c *Client) ReadLine(timeout time.Duration) ([]byte, error) {
	select {
	case line, ok := <-c.lines:
		if !ok {
			return nil, cmp.Or(c.readErr, io.EOF)
		}
		return line, nil
	case <-time.After(timeout):
		return nil, ErrTimeout
	}
}

// Next is the next envelope the server sent, within timeout.
func (c *Client) Next(timeout time.Duration) (protocol.Envelope, error) {
	line, err := c.ReadLine(timeout)
	if err != nil {
		return protocol.Envelope{}, err
	}
	var env protocol.Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return protocol.Envelope{}, fmt.Errorf("prototest: %s: %w", line, err)
	}
	return env, nil
}

// Request sends a request and waits for its reply, keeping the pushes that
// come first for WaitPush. An error reply is returned as its protocol.Error.
func (c *Client) Request(typ protocol.MessageType, body any) (json.RawMessage, error) {
	id, err := c.Send(typ, body)
	if err != nil {
		return nil, err
	}
	for {
		env, err := c.Next(ReplyTimeout)
		if err != nil {
			return nil, fmt.Errorf("no reply to %s: %w", typ, err)
		}
		if env.ID == nil {
			c.backlog = append(c.backlog, env)
			continue
		}
		if *env.ID != id {
			return nil, fmt.Errorf("reply id %d, want %d", *env.ID, id)
		}
		return ReplyOf(env)
	}
}

// WaitPush returns the oldest unconsumed push of type typ, keeping pushes of
// other types for later calls.
func (c *Client) WaitPush(typ protocol.MessageType, timeout time.Duration) (json.RawMessage, error) {
	if i := slices.IndexFunc(c.backlog, func(e protocol.Envelope) bool { return e.Type == typ }); i >= 0 {
		body := c.backlog[i].Body
		c.backlog = slices.Delete(c.backlog, i, i+1)
		return body, nil
	}
	deadline := time.Now().Add(timeout)
	for {
		env, err := c.Next(time.Until(deadline))
		if err != nil {
			return nil, fmt.Errorf("no %s push within %v: %w", typ, timeout, err)
		}
		if env.ID != nil {
			return nil, fmt.Errorf("reply to request %d while waiting for a %s push", *env.ID, typ)
		}
		if env.Type == typ {
			return env.Body, nil
		}
		c.backlog = append(c.backlog, env)
	}
}

// ReplyOf is the body of an ok reply, or the protocol.Error of an error reply.
func ReplyOf(env protocol.Envelope) (json.RawMessage, error) {
	switch env.Type {
	case protocol.TypeOK:
		return env.Body, nil
	case protocol.TypeError:
		var body protocol.ErrorBody
		if err := json.Unmarshal(env.Body, &body); err != nil {
			return nil, err
		}
		return nil, protocol.Error{Code: body.Code, Msg: body.Message}
	}
	return nil, fmt.Errorf("reply of type %s", env.Type)
}

// Serve runs serve on a fresh local listener and returns its address and a
// shutdown, run once however often it is called, that cancels serve's context
// and is the error serve returned, or one if it did not return within wait.
func Serve(t testing.TB, cancel context.CancelFunc, wait time.Duration, serve func(net.Listener) error) (string, func() error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- serve(ln) }()
	return ln.Addr().String(), sync.OnceValue(func() error {
		cancel()
		select {
		case err := <-served:
			return err
		case <-time.After(wait):
			return fmt.Errorf("serve did not return within %v of shutdown", wait)
		}
	})
}

// WaitFor polls done every millisecond until it holds, for at most timeout,
// and reports whether it held.
func WaitFor(timeout time.Duration, done func() bool) bool {
	deadline := time.Now().Add(timeout)
	for !done() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
	return true
}

// WantCode fails t unless err is a protocol.Error with code, and returns its
// message.
func WantCode(t testing.TB, err error, code protocol.ErrorCode) string {
	t.Helper()
	e, ok := errors.AsType[protocol.Error](err)
	if !ok || e.Code != code {
		t.Fatalf("want %s error, got %v", code, err)
	}
	return e.Msg
}
