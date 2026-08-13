package routesmoke

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"
)

// Config identifies one new fixture/evidence pair and bounded Docker campaign.
type Config struct {
	FixtureRoot, EvidenceRoot, ComposeFile, SourceRoot string
	Duration                                           time.Duration
}

// Result is the terminal aggregate result of a local development campaign.
type Result struct {
	Verdict, Reason, EvidenceRoot, EvidenceDigest string
	SourceDigest, ImageID                         string
	Attempts                                      int
	Elapsed                                       time.Duration
	DockerCleanup, FixtureCleanup                 bool
}

// Run executes fresh six-container Route attempts until Duration elapses.
func Run(ctx context.Context, input Config) Result {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	started := time.Now()
	fixture, err := prepare(input)
	if err != nil {
		result := Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot, DockerCleanup: true}
		if ownsFixture(input.FixtureRoot) {
			cleanupErr := removeFixture(input.FixtureRoot)
			result.FixtureCleanup = cleanupErr == nil
			if cleanupErr != nil {
				result.Reason = errors.Join(err, cleanupErr).Error()
			}
		}
		result.Elapsed = time.Since(started)
		return result
	}
	result := runDocker(ctx, input, fixture)
	if cleanupErr := removeFixture(input.FixtureRoot); cleanupErr != nil {
		result.Verdict = "invalid"
		result.Reason = errors.Join(errors.New(result.Reason), cleanupErr).Error()
	} else {
		result.FixtureCleanup = true
	}
	result.Elapsed = time.Since(started)
	return finalizeEvidence(input, result)
}
