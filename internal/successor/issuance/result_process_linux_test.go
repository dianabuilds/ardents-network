//go:build linux

package issuance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

type resultProcessPlan struct {
	Keys, Admission, Results, Batch string
	Binding                         admission.LedgerBinding
	Facts                           admission.Facts
	AfterResponse                   bool
}

func TestResultProcessCrash(t *testing.T) {
	if path := os.Getenv("ARDENTS_RESULT_CRASH_PLAN"); path != "" {
		raw, e := os.ReadFile(path)
		if e != nil {
			os.Exit(2)
		}
		var p resultProcessPlan
		if json.Unmarshal(raw, &p) != nil {
			os.Exit(3)
		}
		ctx := context.Background()
		ledger, e := admission.Open(p.Admission, p.Binding)
		if e != nil {
			os.Exit(4)
		}
		keys, e := Open(ctx, p.Keys, testBinding(1))
		if e != nil {
			os.Exit(5)
		}
		results, e := OpenResults(ctx, p.Results, keys, p.Binding)
		if e != nil {
			os.Exit(6)
		}
		raw, e = os.ReadFile(p.Batch)
		if e != nil {
			os.Exit(7)
		}
		outcome, c := ledger.DebitVerified(ctx, raw, p.Facts, admission.Bootstrap)
		if outcome != admission.Debited {
			os.Exit(8)
		}
		if p.AfterResponse {
			if _, _, e = results.Issue(ctx, c, p.Facts.Now); e != nil {
				os.Exit(9)
			}
		}
		_, _ = os.Stdout.WriteString("ready\n")
		for {
			time.Sleep(time.Hour)
			runtime.KeepAlive(ledger)
			runtime.KeepAlive(keys)
			runtime.KeepAlive(results)
		}
	}
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "after-debit", true: "after-response"}[after], func(t *testing.T) {
			store, v, root := resultOwners(t)
			raw, f, b, verify := resultFixture(t, v, 2, 1, 1)
			ledgerRoot := filepath.Join(t.TempDir(), "admission")
			if e := admission.Initialize(ledgerRoot, b); e != nil {
				t.Fatal(e)
			}
			if e := InitializeResults(t.Context(), root, store, b); e != nil {
				t.Fatal(e)
			}
			keyRoot := store.state.path
			if e := store.Close(); e != nil {
				t.Fatal(e)
			}
			batch := filepath.Join(t.TempDir(), "batch")
			if e := os.WriteFile(batch, raw, 0600); e != nil {
				t.Fatal(e)
			}
			plan := resultProcessPlan{keyRoot, ledgerRoot, root, batch, b, f, after}
			encoded, e := json.Marshal(plan)
			if e != nil {
				t.Fatal(e)
			}
			planFile := filepath.Join(t.TempDir(), "plan")
			if e = os.WriteFile(planFile, encoded, 0600); e != nil {
				t.Fatal(e)
			}
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestResultProcessCrash$")
			command.Env = append(os.Environ(), "ARDENTS_RESULT_CRASH_PLAN="+planFile)
			stdout, e := command.StdoutPipe()
			if e != nil {
				t.Fatal(e)
			}
			if e = command.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
			ready := make([]byte, 6)
			if _, e = io.ReadFull(stdout, ready); e != nil || string(ready) != "ready\n" {
				t.Fatal("child", e, string(ready))
			}
			if _, e := admission.Open(ledgerRoot, b); !errors.Is(e, admission.ErrBusy) {
				t.Fatal(e)
			}
			_ = command.Process.Kill()
			_ = command.Wait()
			ledger, e := admission.Open(ledgerRoot, b)
			if e != nil {
				t.Fatal(e)
			}
			defer ledger.Close()
			outcome, c := ledger.DebitVerified(t.Context(), raw, f, admission.Bootstrap)
			if outcome != admission.AlreadyDebited {
				t.Fatal("lost debit", outcome)
			}
			keys, e := Open(t.Context(), keyRoot, testBinding(1))
			if e != nil {
				t.Fatal(e)
			}
			defer keys.Close()
			results, e := OpenResults(t.Context(), root, keys, b)
			if e != nil {
				t.Fatal(e)
			}
			defer results.Close()
			response, replayed, e := results.Issue(t.Context(), c, f.Now)
			if e != nil || replayed != after {
				t.Fatal(e, replayed)
			}
			verify(response)
		})
	}
}
func TestResultConcurrentIssueAndCancellation(t *testing.T) {
	store, v, root := resultOwners(t)
	raw, f, b, _ := resultFixture(t, v, 1, 1, 1)
	path := filepath.Join(t.TempDir(), "admission")
	if e := admission.Initialize(path, b); e != nil {
		t.Fatal(e)
	}
	ledger, e := admission.Open(path, b)
	if e != nil {
		t.Fatal(e)
	}
	defer ledger.Close()
	outcome, c := ledger.DebitVerified(t.Context(), raw, f, admission.Bootstrap)
	if outcome != admission.Debited {
		t.Fatal(outcome)
	}
	if e := InitializeResults(t.Context(), root, store, b); e != nil {
		t.Fatal(e)
	}
	r, e := OpenResults(t.Context(), root, store, b)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, e := r.Issue(canceled, c, f.Now); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	responses := make(chan []byte, 8)
	fresh := make(chan bool, 8)
	for range 8 {
		wg.Go(func() {
			response, replay, e := r.Issue(t.Context(), c, f.Now)
			if e != nil {
				t.Error(e)
			}
			responses <- response
			fresh <- !replay
		})
	}
	wg.Wait()
	close(responses)
	close(fresh)
	count := 0
	for first := range fresh {
		if first {
			count++
		}
	}
	if count != 1 {
		t.Fatal("duplicate commit", count)
	}
	var before []byte
	for response := range responses {
		if before == nil {
			before = response
		}
		if !bytes.Equal(before, response) {
			t.Fatal("different concurrent result")
		}
	}
	// A real cancellation after response writing starts cannot erase commit.
	root2 := filepath.Join(t.TempDir(), "results")
	if e := InitializeResults(t.Context(), root2, store, b); e != nil {
		t.Fatal(e)
	}
	r2, e := OpenResults(t.Context(), root2, store, b)
	if e != nil {
		t.Fatal(e)
	}
	defer r2.Close()
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	r2.state.fault = func(phase string) error {
		if phase == "response-write" {
			stop()
		}
		return nil
	}
	response, _, e := r2.Issue(ctx, c, f.Now)
	if e != nil || !bytes.Equal(before, response) || ctx.Err() == nil {
		t.Fatal("cancel lost commit", e)
	}
}
