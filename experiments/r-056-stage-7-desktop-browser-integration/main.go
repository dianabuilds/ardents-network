//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var commands = map[string]action{
	"p": toggleDistribution, "u": toggleRegistration, "m": toggleMode,
	"d": toggleBrowser,
	"v": cycleCarrier, "c": connectStream, "a": acceptStream,
	"b": directBrowse, "o": operatingSystemOpen,
}

func main() {
	current := newSession()
	if len(os.Args) > 1 {
		for _, token := range os.Args[1:] {
			current = run(current, token)
		}
		return
	}

	render(current)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("action> ")
		if !scanner.Scan() {
			return
		}
		token := strings.TrimSpace(scanner.Text())
		if token == "q" {
			return
		}
		if token == "h" || token == "?" {
			help()
			continue
		}
		current = run(current, token)
	}
}

func run(current session, token string) session {
	requested, ok := commands[token]
	if !ok {
		fmt.Printf("unknown action %q (use h)\n", token)
		return current
	}
	next := apply(current, requested)
	if err := next.invariantError(); err != nil {
		fmt.Printf("INVARIANT FAILURE: %v\n", err)
		os.Exit(2)
	}
	render(next)
	return next
}

func render(current session) {
	fmt.Println("--- R-056 disposable state model ---")
	fmt.Printf("distribution       %s\n", current.Distribution)
	fmt.Printf("URI registration   %t (optional)\n", current.Registration)
	fmt.Printf("browser mode       %s\n", current.Mode)
	fmt.Printf("default browser    %t\n", current.DefaultBrowser)
	fmt.Printf("Carrier policy     %s\n", current.Carrier)
	if current.Last.Entry == "" {
		fmt.Println("last result        none")
		return
	}
	fmt.Printf("entry              %s\n", current.Last.Entry)
	fmt.Printf("result class       %s\n", current.Last.Class)
	fmt.Printf("claim ceiling      %s\n", current.Last.Claim)
	fmt.Printf("fallback           %s\n", current.Last.Fallback)
	fmt.Printf("host changes       %s\n", current.Last.HostChanges)
	fmt.Printf("detail             %s\n", current.Last.Detail)
}

func help() {
	fmt.Println(`p profile: Installed/Portable   u explicit per-user URI registration
m browser mode: generic/isolated  d default-browser availability
v Carrier policy                  c direct connect byte stream
a direct accept byte stream       b direct browse command
o OS ardents:// handoff           h help   q quit`)
}
