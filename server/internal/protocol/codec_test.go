package protocol_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const testMaxLine = 64 * 1024

func TestReaderSplitsPartialWrites(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = pw.Close() }()
		for _, b := range []byte("{\"id\":1,\"type\":\"ping\",\"body\":{}}\n{\"type\":\"game.update\",\"body\":{}}\n") {
			_, _ = pw.Write([]byte{b})
		}
	}()
	reader := protocol.NewReader(pr, testMaxLine)

	first, err := reader.Next()
	if err != nil || first.Type != protocol.TypePing || first.ID == nil || *first.ID != 1 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	second, err := reader.Next()
	if err != nil || second.Type != protocol.TypeGameUpdate || second.ID != nil {
		t.Fatalf("second = %+v, %v", second, err)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestReaderRejectsLongLine(t *testing.T) {
	line := strings.Repeat("x", testMaxLine+1) + "\n"
	_, err := protocol.NewReader(strings.NewReader(line), testMaxLine).Next()
	if !errors.Is(err, protocol.ErrLineTooLong) {
		t.Fatalf("want ErrLineTooLong, got %v", err)
	}
}

func TestReaderAcceptsLineAtLimit(t *testing.T) {
	prefix := `{"type":"ping","body":"`
	padding := strings.Repeat("x", testMaxLine-len(prefix)-2)
	line := prefix + padding + "\"}\n"
	if len(line) != testMaxLine+1 {
		t.Fatalf("test setup: line is %d bytes", len(line))
	}
	if _, err := protocol.NewReader(strings.NewReader(line), testMaxLine).Next(); err != nil {
		t.Fatal(err)
	}
}

func TestEncodeFrameMatchesEnvelopeEncoding(t *testing.T) {
	id := protocol.RequestID(math.MaxUint64)
	tests := map[string]struct {
		id   *protocol.RequestID
		body any
	}{
		"reply":           {&id, protocol.SeqResult{Seq: 3}},
		"push":            {nil, protocol.Empty{}},
		"null body":       {nil, nil},
		"escaped text":    {&id, protocol.ErrorBody{Message: "<a & b>\u2028\"q\"\n"}},
		"raw nested JSON": {&id, protocol.GameUpdate{View: json.RawMessage(`{ "fen" : "<x>" }`), Events: []json.RawMessage{json.RawMessage(` [1, 2] `)}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(protocol.Envelope{ID: tt.id, Type: protocol.TypeGameUpdate, Body: body})
			if err != nil {
				t.Fatal(err)
			}
			got, err := protocol.EncodeFrame(tt.id, protocol.TypeGameUpdate, tt.body)
			if err != nil || string(got) != string(want)+"\n" {
				t.Fatalf("frame %q, %v; want %q", got, err, string(want)+"\n")
			}
			env, err := protocol.NewReader(bytes.NewReader(got), testMaxLine).Next()
			if err != nil || env.Type != protocol.TypeGameUpdate || (env.ID == nil) != (tt.id == nil) {
				t.Fatalf("frame reads back as %+v, %v", env, err)
			}
		})
	}
}

func TestInfoToWireKeysOptionsByName(t *testing.T) {
	wire := protocol.InfoToWire(game.Info{
		Type:    "chess",
		Options: []game.OptionSpec{{Name: "color", Values: []game.OptionValue{"white", "black"}, Default: "white"}},
	})
	opt := wire.Options["color"]
	if opt.Type != protocol.OptionEnum || opt.Default != "white" || len(opt.Values) != 2 {
		t.Fatalf("option = %+v", opt)
	}
}

func TestInfoToWireEncodesMissingListsAsEmptyArrays(t *testing.T) {
	raw, err := json.Marshal(protocol.InfoToWire(game.Info{Type: "stub", Options: []game.OptionSpec{{Name: "x"}}}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"stub","name":"","players":{"min":0,"max":0},"capabilities":[],"options":{"x":{"type":"enum","values":[],"default":""}}}`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}
}

func TestInfoToWireCarriesTextOptionKind(t *testing.T) {
	raw, err := json.Marshal(protocol.InfoToWire(game.Info{Type: "stub", Options: []game.OptionSpec{
		game.TextOption("fen", 90),
		{Name: "mode", Values: []game.OptionValue{"puzzle"}, Default: "puzzle", MaxLen: 5},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"stub","name":"","players":{"min":0,"max":0},"capabilities":[],"options":{` +
		`"fen":{"type":"text","values":[],"default":"","max_len":90},` +
		`"mode":{"type":"enum","values":["puzzle"],"default":"puzzle"}}}`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}
}
