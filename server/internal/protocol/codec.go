package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

var (
	ErrLineTooLong       = errors.New("protocol: line exceeds maximum length")
	ErrMalformedEnvelope = errors.New("protocol: malformed envelope")
)

const (
	lineDelimiter    = '\n'
	initialBufferLen = 4096
)

type Reader struct {
	scanner *bufio.Scanner
}

func NewReader(r io.Reader, maxLine int) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, min(initialBufferLen, maxLine+1)), maxLine+1)
	return &Reader{scanner: scanner}
}

func (r *Reader) Next() (Envelope, error) {
	for r.scanner.Scan() {
		line := bytes.TrimSpace(r.scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			return Envelope{}, fmt.Errorf("%w: %w", ErrMalformedEnvelope, err)
		}
		return env, nil
	}
	if err := r.scanner.Err(); errors.Is(err, bufio.ErrTooLong) {
		return Envelope{}, ErrLineTooLong
	} else if err != nil {
		return Envelope{}, err
	}
	return Envelope{}, io.EOF
}

// Frame is an envelope encoded as its line. It is never modified once built,
// so one frame can be queued to many sessions.
type Frame []byte

// EncodeFrame encodes body once, straight into the line that marshalling the
// Envelope holding it would give.
func EncodeFrame(id *RequestID, typ MessageType, body any) (Frame, error) {
	e := frameEncoders.Get().(*frameEncoder)
	defer e.release()
	e.buf.WriteByte('{')
	if id != nil {
		e.buf.WriteString(`"id":`)
		e.buf.Write(strconv.AppendUint(e.buf.AvailableBuffer(), uint64(*id), 10))
		e.buf.WriteByte(',')
	}
	e.buf.WriteString(`"type":`)
	if err := e.value(typ); err != nil {
		return nil, fmt.Errorf("protocol: encode type %s: %w", typ, err)
	}
	e.buf.WriteString(`,"body":`)
	if err := e.value(body); err != nil {
		return nil, fmt.Errorf("protocol: encode %s body: %w", typ, err)
	}
	e.buf.WriteByte('}')
	e.buf.WriteByte(lineDelimiter)
	return bytes.Clone(e.buf.Bytes()), nil
}

// frameEncoder builds frames in a buffer kept across them; each frame is
// copied out, since sessions queue it after EncodeFrame returns.
type frameEncoder struct {
	buf bytes.Buffer
	enc *json.Encoder
}

// maxKeptFrameBuffer is the largest buffer a frameEncoder keeps, so one huge
// frame does not pin its memory.
const maxKeptFrameBuffer = 64 << 10

var frameEncoders = sync.Pool{New: func() any {
	e := &frameEncoder{}
	e.enc = json.NewEncoder(&e.buf)
	return e
}}

// value writes v as json.Marshal would, without the newline the Encoder ends it with.
func (e *frameEncoder) value(v any) error {
	if err := e.enc.Encode(v); err != nil {
		return err
	}
	e.buf.Truncate(e.buf.Len() - 1)
	return nil
}

func (e *frameEncoder) release() {
	if e.buf.Cap() > maxKeptFrameBuffer {
		return
	}
	e.buf.Reset()
	frameEncoders.Put(e)
}
