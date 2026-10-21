package hub_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	conformanceDir = "../../../docs/conformance"
	recvTimeout    = 2 * time.Second
	placeholder    = "$"
	// syncID is the id of the ping that flushes what a client received.
	syncID = "999999"
)

// transcriptLine is one line of a docs/conformance transcript: a comment,
// one client sending, receiving, or closing its connection, or the server's
// clock moving on by AdvanceMS, which fires the timers that come due. A
// transcript may open with a header line of Seeds, which its games take in
// order of creation.
type transcriptLine struct {
	Seeds     []game.Seed     `json:"seeds"`
	Comment   string          `json:"comment"`
	Client    string          `json:"client"`
	Send      json.RawMessage `json:"send"`
	Recv      json.RawMessage `json:"recv"`
	Close     bool            `json:"close"`
	AdvanceMS int64           `json:"advance_ms"`
}

func TestConformanceTranscripts(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join(conformanceDir, "*.ndjson"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no transcripts in %s: %v", conformanceDir, err)
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".ndjson"), func(t *testing.T) {
			t.Parallel()
			lines := readTranscript(t, path)
			e := newEnv(t)
			first := 0
			if header := lines[0]; header.Seeds != nil {
				e.seeds = seedQueue(header.Seeds, func() { t.Error("the transcript's games outnumber its seeds") })
				first = 1
			}
			r := &replay{t: t, addr: e.start(t).addr, clock: e.clock, wire: sharedWireSchema(t), clients: map[string]*rawClient{}, bound: map[string]any{}}
			for n := first; n < len(lines); n++ {
				r.step(fmt.Sprintf("%s:%d", filepath.Base(path), n+1), lines[n])
			}
			for _, name := range slices.Sorted(maps.Keys(r.clients)) {
				r.flush(name)
			}
		})
	}
}

// seedQueue hands out seeds in order and calls exhausted for each draw past
// the last. The hub draws from its own goroutines.
func seedQueue(seeds []game.Seed, exhausted func()) func() game.Seed {
	var mu sync.Mutex
	return func() game.Seed {
		mu.Lock()
		defer mu.Unlock()
		if len(seeds) == 0 {
			exhausted()
			return 0
		}
		seed := seeds[0]
		seeds = seeds[1:]
		return seed
	}
}

func readTranscript(t *testing.T, path string) []transcriptLine {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []transcriptLine
	for n, text := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var line transcriptLine
		dec := json.NewDecoder(bytes.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&line); err != nil {
			t.Fatalf("%s:%d: %v", path, n+1, err)
		}
		lines = append(lines, line)
	}
	return lines
}

// replay plays one transcript against a fresh server. bound holds the value
// each placeholder took when first received.
type replay struct {
	t       *testing.T
	addr    string
	clock   *clocktest.FakeClock
	wire    *wireSchema
	clients map[string]*rawClient
	bound   map[string]any
}

func (r *replay) step(at string, line transcriptLine) {
	switch {
	case line.Comment != "":
	case line.Send != nil:
		r.client(line.Client).send(r.t, at, r.substitute(at, decodeJSON(r.t, at, line.Send)))
	case line.Recv != nil:
		r.recv(at, line.Client, decodeJSON(r.t, at, line.Recv))
	case line.Close:
		r.flush(line.Client)
		_ = r.clients[line.Client].Close()
		delete(r.clients, line.Client)
	case line.AdvanceMS > 0:
		r.clock.Advance(time.Duration(line.AdvanceMS) * time.Millisecond)
	default:
		r.t.Fatalf("%s: line does nothing", at)
	}
}

func (r *replay) client(name string) *rawClient {
	if c, ok := r.clients[name]; ok {
		return c
	}
	c := &rawClient{Client: prototest.Dial(r.t, r.addr), sent: map[string]protocol.MessageType{}}
	r.clients[name] = c
	return c
}

// recv takes the first message the client got, in arrival order, that matches want.
func (r *replay) recv(at, name string, want any) {
	c := r.clients[name]
	if c == nil {
		r.t.Fatalf("%s: %s is not connected", at, name)
	}
	deadline := time.Now().Add(recvTimeout)
	for i := 0; ; i++ {
		if i == len(c.backlog) && !r.receive(at, c, deadline) {
			r.t.Fatalf("%s: %s did not receive\n%s\nbut\n%s", at, name, encodeJSON(want), encodeLines(c.backlog))
		}
		bound := maps.Clone(r.bound)
		if matches(want, c.backlog[i], bound) {
			r.bound = bound
			c.backlog = slices.Delete(c.backlog, i, i+1)
			return
		}
	}
}

// receive moves the client's next message into its backlog once it holds to
// the schema, and reports false when none came by deadline.
func (r *replay) receive(at string, c *rawClient, deadline time.Time) bool {
	line, err := c.ReadLine(time.Until(deadline))
	if err != nil {
		return false
	}
	msg, ok := decodeJSON(r.t, at, line).(map[string]any)
	if !ok {
		r.t.Fatalf("%s: not an object: %s", at, line)
	}
	id, _ := msg["id"].(json.Number)
	if err := r.wire.check(msg, c.sent[id.String()]); err != nil {
		r.t.Fatalf("%s: %s breaks the schema: %v", at, line, err)
	}
	c.backlog = append(c.backlog, msg)
	return true
}

// flush fails on any message the client got that the transcript did not expect.
func (r *replay) flush(name string) {
	at := name + " at the end"
	c := r.clients[name]
	c.send(r.t, at, map[string]any{"id": json.Number(syncID), "type": string(protocol.TypePing)})
	r.recv(at, name, map[string]any{"id": json.Number(syncID), "type": placeholder + "sync_type", "body": placeholder + "sync_body"})
	delete(r.bound, "sync_type")
	delete(r.bound, "sync_body")
	if len(c.backlog) > 0 {
		r.t.Fatalf("%s: %s received unexpected\n%s", at, name, encodeLines(c.backlog))
	}
}

func (r *replay) substitute(at string, v any) any {
	switch v := v.(type) {
	case string:
		name, ok := placeholderOf(v)
		if !ok {
			return literalOf(v)
		}
		value, bound := r.bound[name]
		if !bound {
			r.t.Fatalf("%s: %s is sent before it is received", at, v)
		}
		return value
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = r.substitute(at, e)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = r.substitute(at, e)
		}
		return out
	}
	return v
}

// placeholderOf is the name a "$name" string binds; "$$" starts a literal.
func placeholderOf(s string) (string, bool) {
	if strings.HasPrefix(s, placeholder+placeholder) {
		return "", false
	}
	return strings.CutPrefix(s, placeholder)
}

// literalOf is s with a leading "$$" read as one literal "$".
func literalOf(s string) string {
	if strings.HasPrefix(s, placeholder+placeholder) {
		return s[len(placeholder):]
	}
	return s
}

// matches reports whether got is want with each "$name" placeholder taking
// the value bound to name, binding it on first sight. Objects match on
// exactly the same keys.
func matches(want, got any, bound map[string]any) bool {
	switch w := want.(type) {
	case string:
		name, ok := placeholderOf(w)
		if !ok {
			return literalOf(w) == got
		}
		if value, seen := bound[name]; seen {
			return reflect.DeepEqual(value, got)
		}
		bound[name] = got
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for k, e := range w {
			if v, ok := g[k]; !ok || !matches(e, v, bound) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !matches(w[i], g[i], bound) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(want, got)
}

// rawClient is one connection of a transcript. It keeps the messages it got
// that no recv has matched yet, and the type of each request it sent by id.
type rawClient struct {
	*prototest.Client
	backlog []any
	sent    map[string]protocol.MessageType
}

func (c *rawClient) send(t *testing.T, at string, msg any) {
	t.Helper()
	if m, ok := msg.(map[string]any); ok {
		if id, ok := m["id"].(json.Number); ok {
			typ, _ := m["type"].(string)
			c.sent[id.String()] = protocol.MessageType(typ)
		}
	}
	line, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("%s: %v", at, err)
	}
	if err := c.WriteLine(line); err != nil {
		t.Fatalf("%s: %v", at, err)
	}
}

func decodeJSON(t *testing.T, at string, raw []byte) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s: %v", at, err)
	}
	return v
}

func encodeJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}

func encodeLines(vs []any) string {
	return strings.Join(xslices.Map(vs, encodeJSON), "\n")
}

func TestPlaceholdersBindOnceAndDollarDollarIsLiteral(t *testing.T) {
	tests := []struct {
		name      string
		want, got any
		matches   bool
	}{
		{"a reused name binds the same value", []any{"$x", "$x"}, []any{"a", "a"}, true},
		{"a reused name refuses another value", []any{"$x", "$x"}, []any{"a", "b"}, false},
		{"$$ is a literal dollar", "$$x", "$x", true},
		{"$$ does not bind", "$$x", "a", false},
	}
	for _, tc := range tests {
		if got := matches(tc.want, tc.got, map[string]any{}); got != tc.matches {
			t.Errorf("%s: matches %t, want %t", tc.name, got, tc.matches)
		}
	}
}

func TestSeedQueueFailsOnceItRunsOut(t *testing.T) {
	draws := 0
	next := seedQueue([]game.Seed{7, 9}, func() { draws++ })
	if got := []game.Seed{next(), next()}; !slices.Equal(got, []game.Seed{7, 9}) || draws != 0 {
		t.Fatalf("drew %v with %d exhausted, want [7 9] with none", got, draws)
	}
	next()
	if draws != 1 {
		t.Errorf("a third draw from two seeds reported %d exhaustions, want 1", draws)
	}
}
