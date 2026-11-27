package console

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

// Console owns the bounded input and output descriptors of one local conversation.
type Console struct {
	Context    context.Context
	Input      *os.File
	Output     io.Writer
	Diagnostic io.Writer
	files      []*os.File
	cancel     context.CancelFunc
	stop       func() bool
	joined     chan struct{}
	once       sync.Once
}

// Open duplicates inherited files and interrupts them on cancellation.
func Open(ctx context.Context, input *os.File, output, diagnostic io.Writer) (*Console, error) {
	if ctx == nil || input == nil {
		return nil, errors.New("local console unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Minute)
	console := &Console{Context: bounded, Output: output, Diagnostic: diagnostic, cancel: cancel, joined: make(chan struct{})}
	owned, err := openFile(input)
	if err != nil {
		cancel()
		return nil, err
	}
	console.Input = owned
	console.files = append(console.files, owned)
	writers := []*io.Writer{&console.Output, &console.Diagnostic}
	for _, writer := range writers {
		if file, ok := (*writer).(*os.File); ok {
			owned, err = openFile(file)
			if err != nil {
				console.closeFiles()
				cancel()
				return nil, err
			}
			console.files = append(console.files, owned)
			*writer = owned
		}
	}
	console.stop = context.AfterFunc(bounded, func() {
		console.closeFiles()
		close(console.joined)
	})
	return console, nil
}

func (console *Console) closeFiles() {
	for _, file := range console.files {
		_ = file.Close()
	}
}

// Close joins cancellation before releasing the remaining owned descriptors.
func (console *Console) Close() {
	if console == nil {
		return
	}
	console.once.Do(func() {
		if !console.stop() {
			<-console.joined
		}
		console.closeFiles()
		console.cancel()
	})
}
