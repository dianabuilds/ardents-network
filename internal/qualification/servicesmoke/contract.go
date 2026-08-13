package servicesmoke

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"
)

// Config identifies one new private fixture, retained evidence root, and Docker campaign.
type Config struct {
	FixtureRoot, EvidenceRoot, ComposeFile, SourceRoot string
	Duration                                           time.Duration
}

// Result is the terminal aggregate Stage 3 development result.
type Result struct {
	Verdict, Reason, EvidenceRoot, EvidenceDigest string
	SourceCommit, ImageID                         string
	Attempts                                      int
	Elapsed                                       time.Duration
	DockerCleanup, FixtureCleanup                 bool
}

// Run executes complete generation-1 to generation-2 attempts for the duration.
func Run(input Config) Result {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	started := time.Now()
	fixture, err := prepare(input)
	if err != nil {
		result := Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot,
			Elapsed: time.Since(started), DockerCleanup: true}
		if ownsPrivateFixture(input.FixtureRoot) {
			cleanupErr := removePrivateFixture(input.FixtureRoot)
			result.FixtureCleanup = cleanupErr == nil
			if cleanupErr != nil {
				result.Reason = errors.Join(err, cleanupErr).Error()
			}
		}
		return finalize(input, result)
	}
	result := runDocker(ctx, input, fixture)
	if cleanupErr := removePrivateFixture(input.FixtureRoot); cleanupErr != nil {
		result.Verdict = "invalid"
		result.Reason = errors.Join(errors.New(result.Reason), cleanupErr).Error()
	} else {
		result.FixtureCleanup = true
	}
	result.Elapsed = time.Since(started)
	return finalize(input, result)
}
