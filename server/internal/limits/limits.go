package limits

import "time"

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
	const heartbeat = 15 * time.Second
	return Limits{
		LoginDeadline:     10 * time.Second,
		Heartbeat:         heartbeat,
		IdleTimeout:       3 * heartbeat,
		LockoutFailures:   5,
		LockoutWindow:     5 * time.Minute,
		LockoutDuration:   5 * time.Minute,
		MaxConnections:    256,
		WriteQueueFrames:  64,
		MaxLineBytes:      64 * 1024,
		ChallengeTTL:      5 * time.Minute,
		FinishedListLimit: 50,
	}
}
