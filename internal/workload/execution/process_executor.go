package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type processSpec struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Dir     string            `json:"dir,omitempty"`
}

const WorkloadGenerationEnvironment = "ARDENTS_WORKLOAD_GENERATION"

type managedProcess struct {
	cmd      *exec.Cmd
	instance Instance
}

type LocalExecutor struct {
	mu        sync.Mutex
	processes map[string]*managedProcess
}

func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{processes: map[string]*managedProcess{}}
}

func (e *LocalExecutor) Prepare(_ context.Context, req Request) (PreparedWorkload, error) {
	if _, err := parseProcessSpec(req.Config); err != nil {
		return PreparedWorkload{}, err
	}
	return PreparedWorkload{
		WorkloadID: req.WorkloadID,
		Generation: time.Now().UTC().UnixNano(),
		PreparedAt: time.Now().UTC(),
		Handle:     req.Config,
	}, nil
}

func (e *LocalExecutor) Start(_ context.Context, prepared PreparedWorkload) (Instance, error) {
	cfg, err := parseProcessSpec(prepared.Handle)
	if err != nil {
		return Instance{}, err
	}
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if cfg.Dir != "" {
		cmd.Dir = cfg.Dir
	}
	{
		env := make([]string, 0, len(cfg.Env)+1)
		for k, v := range cfg.Env {
			env = append(env, k+"="+v)
		}
		env = append(env, WorkloadGenerationEnvironment+"="+strconv.FormatInt(prepared.Generation, 10))
		cmd.Env = append(cmd.Environ(), env...)
	}
	if err := cmd.Start(); err != nil {
		return Instance{}, err
	}

	instance := Instance{
		WorkloadID: prepared.WorkloadID,
		Generation: prepared.Generation,
		Running:    true,
		PID:        cmd.Process.Pid,
		StartedAt:  time.Now().UTC(),
	}

	e.mu.Lock()
	e.processes[prepared.WorkloadID] = &managedProcess{cmd: cmd, instance: instance}
	e.mu.Unlock()

	go e.wait(prepared.WorkloadID, cmd)
	return instance, nil
}

func (e *LocalExecutor) Stop(_ context.Context, instance Instance) error {
	e.mu.Lock()
	proc := e.processes[instance.WorkloadID]
	e.mu.Unlock()
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return StopProcessByPID(instance.PID)
	}
	err := proc.cmd.Process.Kill()
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already finished") {
		return err
	}
	return WaitForWorkloadStop(func() (Instance, error) {
		return e.Inspect(context.Background(), instance.WorkloadID)
	}, instance.WorkloadID, time.Now().Add(2*time.Second))
}

func (e *LocalExecutor) Inspect(_ context.Context, workloadID string) (Instance, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	proc := e.processes[workloadID]
	if proc == nil {
		return Instance{}, fmt.Errorf("workload %s not found", workloadID)
	}
	return proc.instance, nil
}

func (e *LocalExecutor) wait(workloadID string, cmd *exec.Cmd) {
	err := cmd.Wait()
	finishedAt := time.Now().UTC()
	exitCode := 0
	reason := "exited"
	if err != nil {
		reason = err.Error()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		} else {
			exitCode = -1
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	proc := e.processes[workloadID]
	if proc == nil {
		return
	}
	proc.instance.Running = false
	proc.instance.FinishedAt = finishedAt
	proc.instance.Reason = reason
	proc.instance.PID = 0
	if exitCode != 0 || err == nil {
		proc.instance.ExitCode = new(exitCode)
	}
}

func parseProcessSpec(raw string) (processSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return processSpec{}, fmt.Errorf("missing process config")
	}
	if strings.HasPrefix(raw, "{") {
		var cfg processSpec
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return processSpec{}, fmt.Errorf("invalid process config: %w", err)
		}
		if strings.TrimSpace(cfg.Command) == "" {
			return processSpec{}, fmt.Errorf("missing process command")
		}
		if _, reserved := cfg.Env[WorkloadGenerationEnvironment]; reserved {
			return processSpec{}, fmt.Errorf("process environment key is reserved")
		}
		return cfg, nil
	}

	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return processSpec{}, fmt.Errorf("missing process command")
	}
	return processSpec{Command: parts[0], Args: parts[1:]}, nil
}

func WaitForWorkloadStop(inspect func() (Instance, error), workloadID string, deadline time.Time) error {
	var lastInspectErr error
	for time.Now().Before(deadline) {
		current, inspectErr := inspect()
		if inspectErr == nil && !current.Running {
			return nil
		}
		if inspectErr != nil {
			lastInspectErr = inspectErr
		}
		time.Sleep(10 * time.Millisecond)
	}
	if lastInspectErr != nil {
		return fmt.Errorf("workload %s stop status unavailable: %w", workloadID, lastInspectErr)
	}
	return fmt.Errorf("workload %s process is still running after stop deadline", workloadID)
}
