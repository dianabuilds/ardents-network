package main

import (
	runtimeprocess "ardents/internal/runtime/process"
)

func newRuntime(cfg runtimeprocess.Config) runtimeprocess.NodeRuntime {
	return runtimeprocess.New(cfg)
}
