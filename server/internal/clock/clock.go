// Package clock is the injectable time source rooms and challenges share.
package clock

import "time"

type Timer interface{ Stop() bool }

type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

type System struct{}

func (System) Now() time.Time { return time.Now() }

func (System) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }
