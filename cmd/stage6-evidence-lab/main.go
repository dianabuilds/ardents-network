package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6evidence"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(arguments []string) int {
	if len(arguments) > 0 && arguments[0] == "worker" {
		return runWorker(arguments[1:])
	}
	flags := flag.NewFlagSet("stage6-evidence-lab", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", "", "new empty S6E1 campaign root")
	source := flags.String("source-commit", "", "source commit identity")
	dirty := flags.String("dirty-digest", "", "worktree identity or clean")
	if flags.Parse(arguments) != nil || flags.NArg() != 0 || *root == "" {
		return 2
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stage6-evidence-lab:", err)
		return 1
	}
	if err := stage6evidence.Run(*root, *source, *dirty, executable); err != nil {
		fmt.Fprintln(os.Stderr, "stage6-evidence-lab:", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "complete")
	return 0
}

func runWorker(arguments []string) int {
	flags := flag.NewFlagSet("stage6-evidence-lab worker", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", "", "parent-owned worker staging root")
	if flags.Parse(arguments) != nil || flags.NArg() != 0 || *root == "" {
		return 2
	}
	if err := stage6evidence.RunWorker(*root); err != nil {
		fmt.Fprintln(os.Stderr, "stage6-evidence-lab worker:", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "complete")
	return 0
}
