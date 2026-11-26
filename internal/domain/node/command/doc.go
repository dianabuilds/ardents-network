// Package command adapts Node role invocation and process inspection to
// ardents-next. For a role it reads the bounded input, opens and retains the
// original Network State, executes the typed Node plan and projects the outcome.
// The runtime owns the role's admission and physical completion.
package command
