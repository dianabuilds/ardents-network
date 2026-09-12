package route

import (
	"testing"
	"time"
)

func TestClosedDutyLimitsBoundVerificationAndChildren(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	limits, err := NewClosedDutyLimits(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	var releases []func()
	for range closedVerifyConcurrency {
		release, err := limits.BeginVerification()
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	if _, err := limits.BeginVerification(); err == nil {
		t.Fatal("accepted a fifth concurrent verification")
	}
	for _, release := range releases {
		release()
	}
	for range closedVerifyPerSecond - closedVerifyConcurrency {
		release, err := limits.BeginVerification()
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	if _, err := limits.BeginVerification(); err == nil {
		t.Fatal("accepted verification over the duty rate")
	}
	now = now.Add(time.Second)
	channel, err := limits.reserveChannel()
	if err != nil {
		t.Fatal(err)
	}
	for range closedForwardChildren {
		if err := channel.reserveChild(); err != nil {
			t.Fatal(err)
		}
	}
	if err := channel.reserveChild(); err == nil {
		t.Fatal("accepted child over the parent limit")
	}
	channel.release()
	if err := limits.queue(closedDutyQueueBytes); err != nil {
		t.Fatal(err)
	}
	if err := limits.queue(1); err == nil {
		t.Fatal("accepted bytes over the duty queue")
	}
	limits.dequeue(closedDutyQueueBytes)
	if err := limits.queue(1); err != nil {
		t.Fatal(err)
	}
}
