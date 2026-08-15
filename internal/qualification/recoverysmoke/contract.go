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
	FixtureRoot, EvidenceRoot, ComposeFile, SourceRoot         string
	S41Evidence, S42Evidence, Stage3Evidence, ToolImage, Slice string
	Duration                                                   time.Duration
	Bytes                                                      uint32
	ChunkDelay                                                 string
	SourceCommit                                               string
	Prerequisites                                              []qualificationPrerequisite
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
	value := config{Bytes: 4 << 20, ChunkDelay: "20ms", Slice: "s4.1"}
	flags.StringVar(&value.FixtureRoot, "fixture", "", "new external private recovery fixture root")
	flags.StringVar(&value.EvidenceRoot, "evidence", "", "new retained recovery evidence root")
	flags.StringVar(&value.ComposeFile, "compose", "", "Stage 4 recovery Compose file")
	flags.StringVar(&value.SourceRoot, "source", "", "clean committed repository root")
	flags.StringVar(&value.S41Evidence, "s4.1-evidence", "", "passed S4.1 evidence for the same source")
	flags.StringVar(&value.S42Evidence, "s4.2-evidence", "", "passed S4.2 campaign evidence for the same source")
	flags.StringVar(&value.Stage3Evidence, "stage3-evidence", "", "passed Stage 3 evidence for the same source")
	flags.StringVar(&value.ToolImage, "tool-image", "", "immutable R-025 tooling image for S4.3 netem")
	flags.DurationVar(&value.Duration, "duration", 10*time.Minute, "10m..30m recovery development campaign")
	flags.StringVar(&value.Slice, "slice", "s4.1", "implemented recovery slice: s4.1, s4.2, or s4.3")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, errors.New("recovery-smoke has unexpected positional arguments")
	}
	if value.Slice != "s4.1" && value.Slice != "s4.2" && value.Slice != "s4.3" {
		return config{}, errors.New("recovery-smoke slice is not implemented or authorized")
	}
	if value.Slice != "s4.1" && (value.S41Evidence == "" || value.Stage3Evidence == "") {
		return config{}, errors.New("later recovery slices require explicit passed S4.1 and Stage 3 evidence")
	}
	if value.Slice == "s4.3" && (value.S42Evidence == "" || value.ToolImage == "") {
		return config{}, errors.New("S4.3 requires explicit passed S4.2 evidence and an immutable tooling image")
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
	if input.Slice == "s4.2" || input.Slice == "s4.3" {
		input.Prerequisites, err = loadQualificationPrerequisites(input, commit)
		if err != nil {
			return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot,
				Elapsed: time.Since(started), DockerCleanup: true, FixtureCleanup: true}
		}
	}
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
	if input.Slice == "s4.2" || input.Slice == "s4.3" {
		if err := retainQualificationPrerequisites(input.EvidenceRoot, input.Prerequisites); err != nil {
			cleanupErr := removePrivateFixture(input.FixtureRoot)
			return finalize(input, Result{Verdict: "invalid", Reason: errors.Join(err, cleanupErr).Error(),
				EvidenceRoot: input.EvidenceRoot, Elapsed: time.Since(started), DockerCleanup: true,
				FixtureCleanup: cleanupErr == nil})
		}
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
