package game

import "errors"

var ErrNotYourTurn = errors.New("not your turn")

type IllegalActionError struct{ Msg string }

func (e IllegalActionError) Error() string { return e.Msg }

type BadRequestError struct{ Msg string }

func (e BadRequestError) Error() string { return e.Msg }
