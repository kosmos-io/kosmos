package runtime

import "time"

type Result struct {
	Requeue bool

	RequeueAfter time.Duration
}

func (r *Result) IsZero() bool {
	if r == nil {
		return true
	}
	return *r == Result{}
}
