package harness

import (
	"context"
	"time"
)

type Constrainer struct {
	maxMinutes int
}

func NewConstrainer(maxMinutes int) *Constrainer {
	if maxMinutes <= 0 {
		maxMinutes = 30
	}
	return &Constrainer{maxMinutes: maxMinutes}
}

func (c *Constrainer) WithContext(parent context.Context) context.Context {
	ctx, _ := context.WithTimeout(parent, time.Duration(c.maxMinutes)*time.Minute)
	return ctx
}

func (c *Constrainer) Deadline() time.Duration {
	return time.Duration(c.maxMinutes) * time.Minute
}
