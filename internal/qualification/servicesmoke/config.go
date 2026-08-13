package servicesmoke

import (
	"errors"
	"flag"
	"io"
	"time"
)

// ParseConfig parses the bounded command-line adapter contract.
func ParseConfig(arguments []string) (Config, error) {
	flags := flag.NewFlagSet("service-smoke", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var value Config
	flags.StringVar(&value.FixtureRoot, "fixture", "", "new external private Service fixture root")
	flags.StringVar(&value.EvidenceRoot, "evidence", "", "new retained Service evidence root")
	flags.StringVar(&value.ComposeFile, "compose", "", "Stage 3 Service smoke Compose file")
	flags.StringVar(&value.SourceRoot, "source", "", "clean committed repository root")
	flags.DurationVar(&value.Duration, "duration", 20*time.Minute, "10m..30m local development campaign")
	if err := flags.Parse(arguments); err != nil {
		return Config{}, err
	}
	if flags.NArg() != 0 {
		return Config{}, errors.New("service-smoke has unexpected positional arguments")
	}
	return value, nil
}
