package descriptor

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

// These controls cover observation admission and physical completion only.
// Their held callback is not a successful native Startup or manager fixture.
func TestDescriptorStartupQueuedObservationFreshness(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := &startup{ctx: ctx, parent: ctx, cancel: cancel, ready: true}
		type point struct{ round, image int }
		started := make(chan point, 3)
		releases := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
		image, rounds := 1, 0
		observe := func() error {
			rounds++
			started <- point{rounds, image}
			<-releases[rounds-1]
			return nil
		}
		check := func() <-chan error {
			done := make(chan error, 1)
			go func() { done <- s.checkObservation(observe) }()
			return done
		}
		first := check()
		synctest.Wait()
		if got := <-started; got != (point{1, 1}) {
			t.Fatal("first original observation", got)
		}
		image = 2
		second, third := check(), check()
		synctest.Wait()
		close(releases[0])
		synctest.Wait()
		if err := <-first; err != nil {
			t.Fatal(err)
		}
		if got := <-started; got != (point{2, 2}) {
			t.Fatal("queued requests borrowed an earlier image", got)
		}
		for _, done := range []<-chan error{second, third} {
			select {
			case err := <-done:
				t.Fatal("late request escaped the held fresh observation", err)
			default:
			}
		}
		image = 3
		fourth := check()
		synctest.Wait()
		close(releases[1])
		synctest.Wait()
		for _, done := range []<-chan error{second, third} {
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		}
		if got := <-started; got != (point{3, 3}) {
			t.Fatal("request admitted during I/O reused its result", got)
		}
		select {
		case err := <-fourth:
			t.Fatal("third observation did not retain its waiter", err)
		default:
		}
		close(releases[2])
		synctest.Wait()
		if err := <-fourth; err != nil || rounds != 3 {
			t.Fatal("full fresh observations did not finish", rounds, err)
		}
		s.uses.Wait()
	})
}

func TestDescriptorStartupQueuedObservationClose(t *testing.T) {
	for _, lateFailure := range []bool{false, true} {
		name := "caller"
		if lateFailure {
			name = "caller-and-physical-failure"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				ctx, stop := context.WithCancel(parent)
				defer cancel()
				defer stop()
				s := &startup{ctx: ctx, parent: parent, cancel: stop, ready: true, closeDone: make(chan struct{})}
				h := &Startup{original: s}
				started, release := make(chan struct{}), make(chan struct{})
				physical := errors.New("original held observation failed")
				observations := 0
				observe := func() error {
					observations++
					close(started)
					<-release
					if lateFailure {
						return physical
					}
					return nil
				}
				checks := make([]chan error, 3)
				for i := range checks {
					checks[i] = make(chan error, 1)
					go func() { checks[i] <- s.checkObservation(observe) }()
					if i == 0 {
						<-started
					}
				}
				synctest.Wait()
				cancel()
				closed := make(chan error, 1)
				go func() { closed <- h.Close() }()
				synctest.Wait()
				select {
				case err := <-closed:
					t.Fatal("Close released original physical observation", err)
				default:
				}
				for _, done := range checks {
					select {
					case err := <-done:
						t.Fatal("request returned before original observation joined", err)
					default:
					}
				}
				close(release)
				synctest.Wait()
				for _, done := range checks {
					err := <-done
					if lateFailure {
						if !errors.Is(err, physical) {
							t.Fatal("late original failure lost", err)
						}
					} else if err != context.Canceled {
						t.Fatal("original caller error changed", err)
					}
				}
				joined := <-closed
				if !errors.Is(joined, context.Canceled) || lateFailure && !errors.Is(joined, physical) || h.Close() != joined {
					t.Fatal("joined Close lost original causes", joined)
				}
				if err := s.checkObservation(observe); err == nil || observations != 1 {
					t.Fatal("late request revived observation", observations, err)
				}
			})
		})
	}
}
