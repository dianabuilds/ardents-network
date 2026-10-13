package stock

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// Real blind issuance and native presentation journal; fixture facts isolate
// holder lifetime. They establish no signed Network or successful Route.
func TestHolderRetirementDeniesBeforeOriginalObservationJoins(t *testing.T) {
	for _, mode := range []string{"take-authority", "complete-authority", "complete-final"} {
		t.Run(mode, func(t *testing.T) {
			o, h, presentation := issuedStockFixture(t)
			var attempt Attempt
			var response []byte
			if mode == "complete-authority" || mode == "complete-final" {
				var err error
				attempt, err = o.Begin(nextIntent(h))
				if err != nil {
					t.Fatal(err)
				}
				response = issueFixtureResponse(t, h, attempt)
				defer clear(response)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			observe := o.observe
			var observations atomic.Int32
			gateAt := int32(1)
			if mode == "complete-final" {
				gateAt = 2
			}
			o.observe = func() (admission.AuthorityFacts, time.Time, error) {
				profile, at, err := observe()
				if observations.Add(1) == gateAt {
					close(entered)
					<-release
				}
				return profile, at, err
			}
			work := make(chan error, 1)
			go func() {
				if response != nil {
					work <- attempt.Complete(response, nil)
					return
				}
				raw, err := o.Take(t.Context(), presentation, 2)
				if len(raw) != 0 {
					err = errors.Join(err, errors.New("late token bytes escaped original holder"))
				}
				clear(raw)
				work <- err
			}()
			select {
			case <-entered:
			case err := <-work:
				t.Fatal("original operation did not reach held observation", err)
			}
			closed := make(chan error, 1)
			go func() { closed <- o.Close() }()
			denied, checked := make(chan struct{}), make(chan struct{})
			stop := make(chan struct{})
			go func() {
				defer close(checked)
				for {
					if o.Status().Closed {
						close(denied)
						return
					}
					select {
					case <-stop:
						return
					case <-time.After(time.Millisecond):
					}
				}
			}()
			blocked := false
			select {
			case <-denied:
			case <-time.After(100 * time.Millisecond):
				blocked = true
			}
			early := false
			select {
			case <-closed:
				early = true
				t.Error("holder returned custody before original operation joined")
			default:
			}
			close(stop)
			close(release)
			workErr := <-work
			<-checked
			if !early {
				if err := <-closed; err != nil {
					t.Error(err)
				}
			}
			o.observe = observe
			if blocked || workErr == nil || o.Status().Accepted || o.Status().Pending {
				t.Fatal("foreign observation blocked revoke or late result revived holder", workErr)
			}
		})
	}
}
