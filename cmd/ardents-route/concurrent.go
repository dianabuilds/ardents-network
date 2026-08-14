package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/routeplan"
)

type concurrentTask struct {
	actor      route.Actor
	attachment uint32
	close      func() error
}

type concurrentResult struct {
	task     concurrentTask
	evidence route.Evidence
	runErr   error
	closeErr error
}

func runConcurrent(ctx context.Context, sequence *routeplan.Sequence, encoder *json.Encoder) error {
	tasks, err := collectConcurrent(func() (concurrentTask, bool, error) {
		step, ok, nextErr := sequence.Next()
		return concurrentTask{actor: step.Actor, attachment: step.Attachment, close: step.Close}, ok, nextErr
	})
	if err != nil {
		return err
	}
	return executeConcurrent(ctx, tasks, encoder.Encode, route.Run)
}

func collectConcurrent(next func() (concurrentTask, bool, error)) ([]concurrentTask, error) {
	var tasks []concurrentTask
	for {
		task, ok, err := next()
		if err != nil {
			return nil, errors.Join(fmt.Errorf("construct concurrent Route Attachment: %w", err), closeConcurrent(tasks))
		}
		if !ok {
			return tasks, nil
		}
		tasks = append(tasks, task)
	}
}

func closeConcurrent(tasks []concurrentTask) error {
	var result error
	for index := len(tasks) - 1; index >= 0; index-- {
		if err := tasks[index].close(); err != nil {
			result = errors.Join(result,
				fmt.Errorf("close concurrent Route Attachment %d: %w", tasks[index].attachment, err))
		}
	}
	return result
}

func executeConcurrent(ctx context.Context, tasks []concurrentTask, encode func(any) error,
	run func(context.Context, route.Actor, func(route.Evidence)) (route.Evidence, error)) error {
	results := make(chan concurrentResult, len(tasks))
	var output sync.Mutex
	var outputErr error
	for _, task := range tasks {
		go func(task concurrentTask) {
			ready := func(value route.Evidence) {
				value.Attachment = task.attachment
				output.Lock()
				defer output.Unlock()
				outputErr = errors.Join(outputErr, encode(value))
			}
			observation, runErr := run(ctx, task.actor, ready)
			observation.Attachment = task.attachment
			results <- concurrentResult{task: task, evidence: observation,
				runErr: runErr, closeErr: task.close()}
		}(task)
	}
	completed := make(map[uint32]concurrentResult, len(tasks))
	for range tasks {
		result := <-results
		completed[result.task.attachment] = result
	}
	successes := 0
	var terminalErr error
	for attachment := uint32(1); attachment <= uint32(len(tasks)); attachment++ {
		result := completed[attachment]
		if result.runErr == nil {
			successes++
		}
		if result.closeErr != nil {
			terminalErr = errors.Join(terminalErr,
				fmt.Errorf("close concurrent Route Attachment %d: %w", attachment, result.closeErr))
		}
		if err := encode(result.evidence); err != nil {
			terminalErr = errors.Join(terminalErr,
				fmt.Errorf("encode concurrent Route Attachment %d: %w", attachment, err))
		}
	}
	if outputErr != nil {
		terminalErr = errors.Join(terminalErr, fmt.Errorf("encode concurrent Route readiness: %w", outputErr))
	}
	if terminalErr != nil {
		return terminalErr
	}
	if successes == 0 {
		return errors.New("every concurrent publisher Attachment failed")
	}
	return nil
}
