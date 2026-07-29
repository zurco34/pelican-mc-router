package actioncontrol

import (
	"testing"
	"time"
)

func TestLimiterIsBoundedByActionClass(t *testing.T) {
	limiter := New(time.Second)
	now := time.Unix(0, 0)
	if !limiter.Allow(ActionSetup, now) || limiter.Allow(ActionSetup, now) {
		t.Fatal("setup action limit was not enforced")
	}
	if !limiter.Allow(ActionSettings, now) {
		t.Fatal("one action class affected another")
	}
	if !limiter.Allow(ActionSetup, now.Add(time.Second)) {
		t.Fatal("setup action did not recover after interval")
	}
}
