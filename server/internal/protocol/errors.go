package protocol

import "fmt"

// Error is a server-core failure that reaches the client as its code and message.
type Error struct {
	Code ErrorCode
	Msg  string
}

func (e Error) Error() string { return e.Msg }

func BadRequestf(format string, args ...any) Error {
	return Error{Code: CodeBadRequest, Msg: fmt.Sprintf(format, args...)}
}

func NotFoundf(format string, args ...any) Error {
	return Error{Code: CodeNotFound, Msg: fmt.Sprintf(format, args...)}
}
