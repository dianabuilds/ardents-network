package execution

import (
	"context"
	"errors"
	"time"
)

func (e *DockerExecutor) failCreatedContainer(id string, cause error) (Instance, error) {
	stopTimeout := e.stopTimeout
	if stopTimeout <= 0 {
		stopTimeout = 10 * time.Second
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), stopTimeout+5*time.Second)
	defer cancel()
	cleanupErr := e.stopAndRemoveContainer(cleanupCtx, id)
	return Instance{}, errors.Join(cause, cleanupErr)
}
