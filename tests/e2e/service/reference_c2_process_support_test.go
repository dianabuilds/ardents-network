//go:build referencec2

package service_test

import (
	"context"
	"errors"
	"os/exec"
)

type killableCommand struct {
	command *exec.Cmd
	result  <-chan commandResult
}

func startKillableCommand(ctx context.Context, root, binary string, arguments ...string) *killableCommand {
	result := make(chan commandResult, 1)
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = root
	capture := new(commandCapture)
	command.Stdout = commandStreamCapture{capture: capture}
	command.Stderr = commandStreamCapture{capture: capture, stderr: true}
	if err := command.Start(); err != nil {
		result <- capture.result(err)
		close(result)
		return &killableCommand{command: command, result: result}
	}
	go func() {
		result <- capture.result(command.Wait())
		close(result)
	}()
	return &killableCommand{command: command, result: result}
}

func (running *killableCommand) Kill() error {
	if running == nil || running.command == nil || running.command.Process == nil {
		return errors.New("killable command is unavailable")
	}
	return running.command.Process.Kill()
}
