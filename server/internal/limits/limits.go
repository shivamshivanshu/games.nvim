package limits

import "time"

const (
	LoginDeadline     = 10 * time.Second
	Heartbeat         = 15 * time.Second
	IdleTimeout       = 3 * Heartbeat
	LockoutFailures   = 5
	LockoutWindow     = 5 * time.Minute
	LockoutDuration   = 5 * time.Minute
	MaxConnections    = 256
	WriteQueueFrames  = 64
	MaxLineBytes      = 64 * 1024
	ChallengeTTL      = 5 * time.Minute
	FinishedListLimit = 50
)

type Limits struct {
	LoginDeadline     time.Duration
	Heartbeat         time.Duration
	IdleTimeout       time.Duration
	LockoutFailures   int
	LockoutWindow     time.Duration
	LockoutDuration   time.Duration
	MaxConnections    int
	WriteQueueFrames  int
	MaxLineBytes      int
	ChallengeTTL      time.Duration
	FinishedListLimit int
}

func Default() Limits {
	return Limits{
		LoginDeadline:     LoginDeadline,
		Heartbeat:         Heartbeat,
		IdleTimeout:       IdleTimeout,
		LockoutFailures:   LockoutFailures,
		LockoutWindow:     LockoutWindow,
		LockoutDuration:   LockoutDuration,
		MaxConnections:    MaxConnections,
		WriteQueueFrames:  WriteQueueFrames,
		MaxLineBytes:      MaxLineBytes,
		ChallengeTTL:      ChallengeTTL,
		FinishedListLimit: FinishedListLimit,
	}
}
