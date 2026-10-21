package game

import (
	"errors"
	"fmt"
)

var ErrNotYourTurn = errors.New("not your turn")

type IllegalActionError struct{ Msg string }

func (e IllegalActionError) Error() string { return e.Msg }

type BadRequestError struct{ Msg string }

func (e BadRequestError) Error() string { return e.Msg }

func Illegalf(format string, args ...any) error {
	return IllegalActionError{Msg: fmt.Sprintf(format, args...)}
}

func BadRequestf(format string, args ...any) error {
	return BadRequestError{Msg: fmt.Sprintf(format, args...)}
}

// UnknownAction rejects an action whose kind the state does not take.
func UnknownAction[K ~string](kind K) error {
	return IllegalActionError{Msg: "unknown action: " + string(kind)}
}
