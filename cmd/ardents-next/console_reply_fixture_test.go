package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Native console fixtures use zero-buffer io.Pipe. A streaming JSON decoder
// may return at a buffer boundary before consuming Encoder's final newline.
// Consume the entire reply record before writing the next console command;
// otherwise the original console writer and command writer can deadlock.
func readConsoleReply(reader *bufio.Reader, reply any) error {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return err
	}
	if len(line) > 64<<10 {
		return errors.New("console fixture reply exceeds its finite bound")
	}
	return json.Unmarshal(line, reply)
}

func TestConsoleReplyFixtureJoinsCompleteLine(t *testing.T) {
	for _, size := range []int{512, 4096} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			// These independently assembled JSON lengths hit both the streaming
			// decoder and buffered-reader boundaries. No domain success is faked.
			value := struct {
				Payload string `json:"payload"`
			}{}
			empty, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			value.Payload = strings.Repeat("x", size-len(empty))
			encoded, err := json.Marshal(value)
			if err != nil || len(encoded) != size {
				t.Fatal("independent JSON boundary length", len(encoded), err)
			}
			input, output := io.Pipe()
			finished := make(chan error, 1)
			go func() { finished <- json.NewEncoder(output).Encode(value) }()
			joined := false
			t.Cleanup(func() {
				_ = input.Close()
				_ = output.Close()
				if !joined {
					<-finished
				}
			})
			var reply struct {
				Payload string `json:"payload"`
			}
			if err := readConsoleReply(bufio.NewReader(input), &reply); err != nil {
				t.Fatal(err)
			}
			if reply.Payload != value.Payload {
				t.Fatal("reply bytes changed")
			}
			select {
			case err := <-finished:
				joined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("original reply writer still awaits its final newline")
			}
		})
	}
}
