// ardents-browser-entry is the deliberately narrow native host used by the
// selected Firefox alpha Browser Entry. Its separately invoked participant
// install/remove commands own only native-manifest registration; it is not an
// Endpoint launcher or a browser installer.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, input io.Reader, output io.Writer) error {
	if len(arguments) > 0 {
		switch arguments[0] {
		case "install":
			return installBrowserEntry(arguments[1:], output)
		case "remove":
			return removeBrowserEntry(arguments[1:], output)
		}
	}
	statePath, err := nativeHostStatePath(arguments)
	if err != nil {
		return err
	}
	return browserentry.ServeNativeHost(input, output, statePath)
}

// nativeHostStatePath accepts Firefox's manifest invocation with no arguments.
// An explicit native-host command remains available only for a qualification or
// a bounded local operator invocation that supplies an absolute state path.
func nativeHostStatePath(arguments []string) (string, error) {
	if len(arguments) == 0 {
		return browserentry.DefaultStatePath()
	}
	if arguments[0] != "native-host" {
		return "", errors.New("usage: ardents-browser-entry [native-host --state PATH]")
	}
	flags := flag.NewFlagSet("native-host", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var statePath string
	flags.StringVar(&statePath, "state", "", "Endpoint-owned Browser Entry state file")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return "", errors.New("browser Entry native-host arguments are invalid")
	}
	if statePath == "" {
		var defaultErr error
		statePath, defaultErr = browserentry.DefaultStatePath()
		if defaultErr != nil {
			return "", defaultErr
		}
	}
	if !filepath.IsAbs(statePath) {
		return "", errors.New("browser Entry native-host arguments are invalid")
	}
	return statePath, nil
}
