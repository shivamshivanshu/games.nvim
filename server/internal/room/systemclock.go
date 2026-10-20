package room

import "time"

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func (SystemClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }
