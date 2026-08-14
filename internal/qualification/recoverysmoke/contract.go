package recoverysmoke

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"time"
)

type config struct {
	FixtureRoot, EvidenceRoot, ComposeFile, SourceRoot string
	Duration                                           time.Duration
	Bytes                                              uint32
	ChunkDelay                                         string
	SourceCommit                                       string
}

// Result is one terminal Stage 4 development result.
type Result struct {
	Verdict, Reason, EvidenceRoot, EvidenceDigest string
	SourceCommit, ImageID                         string
	Attempts                                      int
	Elapsed                                       time.Duration
	DockerCleanup, FixtureCleanup                 bool
	attemptFiles                                  []string
	dockerProject, imageTag                       string
}

func parseConfig(arguments []string) (config, error) {
	flags := flag.NewFlagSet("recovery-smoke", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	value := config{Bytes: 4 << 20, ChunkDelay: "8ms"}
	flags.StringVar(&value.FixtureRoot, "fixture", "", "new external private recovery fixture root")
	flags.StringVar(&value.EvidenceRoot, "evidence", "", "new retained recovery evidence root")
	flags.StringVar(&value.ComposeFile, "compose", "", "Stage 4 recovery Compose file")
	flags.StringVar(&value.SourceRoot, "source", "", "clean committed repository root")
	flags.DurationVar(&value.Duration, "duration", 10*time.Minute, "10m..30m recovery development campaign")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, errors.New("recovery-smoke has unexpected positional arguments")
	}
	return value, nil
}

// Execute parses and runs one public recovery-smoke invocation.
func Execute(arguments []string) (Result, error) {
	config, err := parseConfig(arguments)
	if err != nil {
		return Result{}, err
	}
	return run(config), nil
}

func run(input config) Result {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now()
	commit, err := cleanCommit(ctx, input.SourceRoot)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot,
			Elapsed: time.Since(started), DockerCleanup: true, FixtureCleanup: true}
	}
	input.SourceCommit = commit
	fixture, err := prepare(input)
	if err != nil {
		result := Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot,
			Elapsed: time.Since(started), DockerCleanup: true}
		if ownsPrivateFixture(input.FixtureRoot) {
			cleanupErr := removePrivateFixture(input.FixtureRoot)
			result.FixtureCleanup = cleanupErr == nil
			result.Reason = errors.Join(err, cleanupErr).Error()
		}
		return finalize(input, result)
	}
	result := runDocker(ctx, input, fixture)
	if !result.FixtureCleanup {
		if err := removePrivateFixture(input.FixtureRoot); err != nil {
			result.Verdict, result.Reason = "invalid", errors.Join(errors.New(result.Reason), err).Error()
		} else {
			result.FixtureCleanup = true
		}
	}
	result.Elapsed = time.Since(started)
	return finalize(input, result)
}
