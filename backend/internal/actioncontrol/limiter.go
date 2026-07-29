// Package actioncontrol provides bounded, action-class request throttling.
package actioncontrol

import (
	"sync"
	"time"
)

type Action uint8

const (
	ActionSetup Action = iota
	ActionSettings
	ActionReconcile
)

type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     [3]time.Time
}

func New(interval time.Duration) *Limiter {
	return &Limiter{interval: interval}
}

func (l *Limiter) Allow(action Action, now time.Time) bool {
	if l == nil || l.interval <= 0 || action > ActionReconcile {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Before(l.next[action]) {
		return false
	}
	l.next[action] = now.Add(l.interval)
	return true
}
