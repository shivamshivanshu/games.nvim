package protocol_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"games.nvim/server/game"
	"games.nvim/server/games/chess"
	"games.nvim/server/internal/protocol"
)

const testMaxLine = 64 * 1024

func TestReaderSplitsPartialWrites(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for _, b := range []byte("{\"id\":1,\"type\":\"ping\",\"body\":{}}\n{\"type\":\"game.update\",\"body\":{}}\n") {
			pw.Write([]byte{b})
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

func TestWriteEnvelopeRoundTrips(t *testing.T) {
	id := uint64(7)
	var buf bytes.Buffer
	in := protocol.Envelope{ID: &id, Type: protocol.TypeOK, Body: []byte(`{"seq":3}`)}
	if err := protocol.WriteEnvelope(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := protocol.NewReader(&buf, testMaxLine).Next()
	if err != nil || *out.ID != id || out.Type != in.Type || string(out.Body) != `{"seq":3}` {
		t.Fatalf("out = %+v, %v", out, err)
	}
}

func TestEncodeEventAddsKind(t *testing.T) {
	got, err := protocol.EncodeEvent(chess.MovedEvent{By: "white", SAN: "e4"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"by":"white","kind":"moved","san":"e4"}`; string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestInfoToWireKeysOptionsByName(t *testing.T) {
	wire := protocol.InfoToWire(game.Info{
		Type:    "chess",
		Options: []game.OptionSpec{{Name: "color", Values: []string{"white", "black"}, Default: "white"}},
	})
	opt := wire.Options["color"]
	if opt.Type != protocol.OptionEnum || opt.Default != "white" || len(opt.Values) != 2 {
		t.Fatalf("option = %+v", opt)
	}
}
