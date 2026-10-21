package hub_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
)

const pushTimeout = 5 * time.Second

func call[Resp any](t *testing.T, c *prototest.Client, typ protocol.MessageType, body any) Resp {
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

func waitPush[Body any](t *testing.T, c *prototest.Client, typ protocol.MessageType) Body {
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
