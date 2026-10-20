// Package prototest is the client end of the wire protocol for tests.
package prototest

import (
	"io"
	"net"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

// Dial connects to addr, closing the connection on t's cleanup, and reads it
// with the server's default line limit.
func Dial(t testing.TB, addr string) (net.Conn, *protocol.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, protocol.NewReader(conn, limits.Default().MaxLineBytes)
}

// Send writes the request id of type typ carrying body as JSON.
func Send(w io.Writer, id protocol.RequestID, typ protocol.MessageType, body any) error {
	frame, err := protocol.EncodeFrame(&id, typ, body)
	if err != nil {
		return err
	}
	_, err = w.Write(frame)
	return err
}
