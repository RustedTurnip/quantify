package quantify

import "time"

type clock interface {
	now() time.Time
}

type realClock struct {}

func (rc *realClock) now() time.Time {
	return time.Now()
}