package processfixture

import (
	"bufio"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

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
			if err := ReadConsoleReply(bufio.NewReader(input), &reply); err != nil {
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

func TestConsoleReplyFixtureBoundsRecordBeforeDecoding(t *testing.T) {
	const limit = 64 << 10
	for _, test := range []struct {
		name string
		line string
		want error
	}{
		{"full record", "0" + strings.Repeat(" ", limit-2) + "\n", nil},
		{"oversized record", "0" + strings.Repeat(" ", limit-1) + "\n", errConsoleReplyTooLong},
		{"incomplete record", "0", io.EOF},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := 42
			reader := bufio.NewReader(strings.NewReader(test.line))
			if err := ReadConsoleReply(reader, &value); err != test.want {
				t.Fatalf("read error = %v, want %v", err, test.want)
			}
			want := 42
			if test.want == nil {
				want = 0
			}
			if value != want {
				t.Fatalf("decoded value = %d, want %d", value, want)
			}
		})
	}
}
