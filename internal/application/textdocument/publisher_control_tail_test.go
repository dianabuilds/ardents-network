//go:build linux

package textdocument_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// This fixture isolates consumer cleanup ordering; its supplied outcome is not
// Service authority or proof of an authenticated Connection. The native profile
// separately exercises the same consumer with genuine workers and Route.
type retainedPublisherFixture struct {
	*publisherFixture
	joined  chan struct{}
	waiting chan struct{}
	once    sync.Once
}

func (fixture *retainedPublisherFixture) Joined() <-chan struct{} {
	fixture.once.Do(func() { close(fixture.waiting) })
	return fixture.joined
}

type joinedPublisherCleanupFixture struct {
	*retainedPublisherFixture
	physical error
}

func (fixture *joinedPublisherCleanupFixture) JoinedCleanup() error {
	select {
	case <-fixture.closed:
		return fixture.physical
	default:
		return errors.New("original fixture cleanup has not joined")
	}
}

func TestPublisherTailCancellationKeepsLogicalAndPhysicalResultsSeparate(t *testing.T) {
	physicalFailure := errors.New("original physical Service cleanup failed")
	for _, physical := range []error{nil, physicalFailure} {
		name := "logical-cancellation"
		if physical != nil {
			name = "physical-failure"
		}
		t.Run(name, func(t *testing.T) {
			harness := startPublisherHarness(t, []byte("original snapshot"), nil)
			fixture := &joinedPublisherCleanupFixture{retainedPublisherFixture: &retainedPublisherFixture{
				publisherFixture: newPublisherFixture(publisherRequest(), nil), joined: make(chan struct{}), waiting: make(chan struct{})}, physical: physical}
			fixture.closeErr = context.Canceled
			harness.admit(t, fixture)
			select {
			case <-fixture.waiting:
			case <-time.After(time.Second):
				t.Fatal("Publisher did not retain the original tail")
			}
			close(fixture.joined)
			if physical == nil {
				harness.drain(t)
				fixture.assertResponse(t, []byte("original snapshot"))
			} else {
				close(harness.incoming)
				select {
				case <-harness.finished:
					if !errors.Is(harness.err, physicalFailure) {
						t.Fatal("Publisher erased the actual physical failure", harness.err)
					}
				case <-time.After(time.Second):
					t.Fatal("physical failure did not finish the Publisher")
				}
			}
			if fixture.Close() != context.Canceled {
				t.Fatal("physical cleanup classification erased the logical result")
			}
		})
	}
}

func TestPublisherRetainsBothJoinedServiceCleanupFailures(t *testing.T) {
	harness := startPublisherHarness(t, []byte("original snapshot"), nil)
	failures := []error{errors.New("first original physical failure"), errors.New("second late physical failure")}
	var fixtures []*joinedPublisherCleanupFixture
	for _, failure := range failures {
		fixture := &joinedPublisherCleanupFixture{retainedPublisherFixture: &retainedPublisherFixture{
			publisherFixture: newPublisherFixture(publisherRequest(), nil), joined: make(chan struct{}), waiting: make(chan struct{})}, physical: failure}
		harness.admit(t, fixture)
		select {
		case <-fixture.waiting:
		case <-time.After(time.Second):
			t.Fatal("original Service did not retain its tail")
		}
		fixtures = append(fixtures, fixture)
	}
	for _, fixture := range fixtures {
		close(fixture.joined)
	}
	select {
	case <-harness.finished:
		for _, failure := range failures {
			if !errors.Is(harness.err, failure) {
				t.Fatal("Publisher erased an original joined failure", harness.err, failure)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("Publisher did not join both failed Services")
	}
}

func TestPublisherCompletedBytesRetainOriginalControlTail(t *testing.T) {
	for _, termination := range []string{"joined", "cancelled"} {
		t.Run(termination, func(t *testing.T) {
			harness := startPublisherHarness(t, []byte("original snapshot"), nil)
			fixture := &retainedPublisherFixture{publisherFixture: newPublisherFixture(publisherRequest(), nil),
				joined: make(chan struct{}), waiting: make(chan struct{})}
			harness.admit(t, fixture)
			select {
			case <-fixture.waiting:
			case <-fixture.closed:
				t.Fatal("Application completion closed an unjoined original control tail")
			case <-time.After(time.Second):
				t.Fatal("Publisher did not retain completed control tail")
			}
			select {
			case <-fixture.closed:
				t.Fatal("retained tail closed before join or cancellation")
			default:
			}
			if termination == "joined" {
				close(fixture.joined)
				harness.drain(t)
			} else {
				harness.cancel()
				select {
				case <-harness.finished:
					if !errors.Is(harness.err, context.Canceled) {
						t.Fatal("Publisher lost original cancellation", harness.err)
					}
				case <-time.After(time.Second):
					t.Fatal("cancellation did not interrupt and join retained tail")
				}
			}
			select {
			case <-fixture.closed:
			default:
				t.Fatal("Publisher did not close original joined stream")
			}
		})
	}
}
