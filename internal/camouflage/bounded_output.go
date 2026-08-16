package camouflage

import (
	"bytes"
	"sync"
)

type boundedOutput struct {
	mu       sync.Mutex
	content  bytes.Buffer
	limit    int
	exceeded bool
}

func (output *boundedOutput) Write(content []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	remaining := output.limit - output.content.Len()
	if remaining > len(content) {
		remaining = len(content)
	}
	if remaining > 0 {
		_, _ = output.content.Write(content[:remaining])
	}
	if remaining < len(content) {
		output.exceeded = true
	}
	return len(content), nil
}

func (output *boundedOutput) bytes() []byte {
	output.mu.Lock()
	defer output.mu.Unlock()
	return bytes.Clone(output.content.Bytes())
}

func (output *boundedOutput) tooLarge() bool {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.exceeded
}
