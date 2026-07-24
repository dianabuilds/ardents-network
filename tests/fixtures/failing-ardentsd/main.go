package main

import (
	"encoding/json"
	"os"
	"time"

	"ardents/internal/buildinfo"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		_ = json.NewEncoder(os.Stdout).Encode(buildinfo.Current())
		return
	}
	time.Sleep(2 * time.Second)
	os.Exit(70)
}
