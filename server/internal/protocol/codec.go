package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

var (
	ErrLineTooLong       = errors.New("protocol: line exceeds maximum length")
	ErrMalformedEnvelope = errors.New("protocol: malformed envelope")
)

const (
	lineDelimiter    = '\n'
	initialBufferLen = 4096
	frameOverhead    = len(`{"id":18446744073709551615,"type":,"body":}`) + 1
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
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("protocol: encode %s body: %w", typ, err)
	}
	quotedType, err := json.Marshal(typ)
	if err != nil {
		return nil, fmt.Errorf("protocol: encode type %s: %w", typ, err)
	}
	frame := make(Frame, 0, len(raw)+len(quotedType)+frameOverhead)
	frame = append(frame, '{')
	if id != nil {
		frame = strconv.AppendUint(append(frame, `"id":`...), uint64(*id), 10)
		frame = append(frame, ',')
	}
	frame = append(append(frame, `"type":`...), quotedType...)
	frame = append(append(frame, `,"body":`...), raw...)
	return append(frame, '}', lineDelimiter), nil
}
