package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Next returns io.EOF at end of stream and skips blank lines.
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

func WriteEnvelope(w io.Writer, e Envelope) error {
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("protocol: encode envelope: %w", err)
	}
	_, err = w.Write(append(line, lineDelimiter))
	return err
}
