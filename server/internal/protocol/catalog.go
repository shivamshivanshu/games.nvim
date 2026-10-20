package protocol

import "reflect"

// RequestSpec is a request type with the bodies it and its ok reply carry.
type RequestSpec struct {
	Type  MessageType
	Body  reflect.Type
	Reply reflect.Type
}

// PushSpec is a push type with the body it carries.
type PushSpec struct {
	Type MessageType
	Body reflect.Type
}

func request[Body, Reply any](typ MessageType) RequestSpec {
	return RequestSpec{Type: typ, Body: reflect.TypeFor[Body](), Reply: reflect.TypeFor[Reply]()}
}

func push[Body any](typ MessageType) PushSpec {
	return PushSpec{Type: typ, Body: reflect.TypeFor[Body]()}
}

// Requests lists every request the server answers.
var Requests = []RequestSpec{
	request[LoginRequest, LoginResult](TypeAuthLogin),
	request[Empty, Empty](TypePing),
	request[Empty, CatalogResult](TypeCatalogList),
	request[Empty, UserListResult](TypeUserList),
	request[ChallengeCreateRequest, ChallengeCreateResult](TypeChallengeCreate),
	request[ChallengeRef, Empty](TypeChallengeAccept),
	request[ChallengeRef, Empty](TypeChallengeDecline),
	request[ChallengeRef, Empty](TypeChallengeCancel),
	request[Empty, ChallengeListResult](TypeChallengeList),
	request[GameListRequest, GameListResult](TypeGameList),
	request[GameSnapshotRequest, GameUpdate](TypeGameSnapshot),
	request[GameActRequest, SeqResult](TypeGameAct),
	request[GameControlRequest, SeqResult](TypeGameControl),
	request[GameStartRequest, GameUpdate](TypeGameStart),
	request[StatsRequest, StatsResult](TypeStatsGet),
	request[GameVerifyRequest, Empty](TypeGameVerify),
}

// Pushes lists every message the server sends unasked.
var Pushes = []PushSpec{
	push[Challenge](TypeChallengeChanged),
	push[GameUpdate](TypeGameUpdate),
}

// ErrorCodes lists every code an error reply carries.
var ErrorCodes = []ErrorCode{
	CodeBadRequest, CodeVersionMismatch, CodeUnauthenticated, CodeAuthFailed, CodeNotFound,
	CodeNotYourTurn, CodeIllegalAction, CodeStale, CodeInternal, CodeSummaryMismatch,
}
