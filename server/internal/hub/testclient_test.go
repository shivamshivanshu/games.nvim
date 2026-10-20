package hub_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const (
	replyTimeout = 5 * time.Second
	pushTimeout  = 5 * time.Second
	pushBacklog  = 256
)

type errorReply protocol.ErrorBody

func (e errorReply) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// testClient is a raw NDJSON client. It must be used from one goroutine; a
// background reader only routes replies and pushes onto channels.
type testClient struct {
	conn    net.Conn
	nextID  protocol.RequestID
	replies chan protocol.Envelope
	pushes  chan protocol.Envelope
	backlog []protocol.Envelope
}

func dial(t *testing.T, addr string) *testClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	c := &testClient{
		conn:    conn,
		replies: make(chan protocol.Envelope, 1),
		pushes:  make(chan protocol.Envelope, pushBacklog),
	}
	go c.readLoop()
	t.Cleanup(func() { _ = conn.Close() })
	return c
}

func (c *testClient) readLoop() {
	reader := protocol.NewReader(c.conn, 1<<20)
	for {
		env, err := reader.Next()
		if err != nil {
			close(c.replies)
			close(c.pushes)
			return
		}
		if env.ID != nil {
			c.replies <- env
		} else {
			c.pushes <- env
		}
	}
}

func (c *testClient) Request(typ protocol.MessageType, body any) (json.RawMessage, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	c.nextID++
	id := c.nextID
	if err := protocol.WriteEnvelope(c.conn, protocol.Envelope{ID: &id, Type: typ, Body: raw}); err != nil {
		return nil, err
	}
	select {
	case env, ok := <-c.replies:
		if !ok {
			return nil, errors.New("connection closed")
		}
		if *env.ID != id {
			return nil, fmt.Errorf("reply id %d, want %d", *env.ID, id)
		}
		if env.Type == protocol.TypeError {
			var reply errorReply
			if err := json.Unmarshal(env.Body, &reply); err != nil {
				return nil, err
			}
			return nil, reply
		}
		return env.Body, nil
	case <-time.After(replyTimeout):
		return nil, fmt.Errorf("no reply to %s", typ)
	}
}

// WaitPush returns the oldest unconsumed push of the given type, keeping
// pushes of other types for later calls.
func (c *testClient) WaitPush(typ protocol.MessageType, timeout time.Duration) (json.RawMessage, error) {
	if i := slices.IndexFunc(c.backlog, func(e protocol.Envelope) bool { return e.Type == typ }); i >= 0 {
		body := c.backlog[i].Body
		c.backlog = slices.Delete(c.backlog, i, i+1)
		return body, nil
	}
	deadline := time.After(timeout)
	for {
		select {
		case env, ok := <-c.pushes:
			if !ok {
				return nil, fmt.Errorf("connection closed waiting for %s", typ)
			}
			if env.Type == typ {
				return env.Body, nil
			}
			c.backlog = append(c.backlog, env)
		case <-deadline:
			return nil, fmt.Errorf("no %s push within %v", typ, timeout)
		}
	}
}

func call[Resp any](t *testing.T, c *testClient, typ protocol.MessageType, body any) Resp {
	t.Helper()
	raw, err := c.Request(typ, body)
	if err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
	var resp Resp
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("%s: decode %s: %v", typ, raw, err)
	}
	return resp
}

func waitPush[Body any](t *testing.T, c *testClient, typ protocol.MessageType) Body {
	t.Helper()
	raw, err := c.WaitPush(typ, pushTimeout)
	if err != nil {
		t.Fatal(err)
	}
	var body Body
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("%s: decode %s: %v", typ, raw, err)
	}
	return body
}

func wantCode(t *testing.T, err error, code protocol.ErrorCode) errorReply {
	t.Helper()
	var reply errorReply
	if !errors.As(err, &reply) || reply.Code != code {
		t.Fatalf("want %s error, got %v", code, err)
	}
	return reply
}
