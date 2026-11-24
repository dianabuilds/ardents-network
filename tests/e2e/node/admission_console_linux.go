package node

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

// AdmissionReply is the bounded reply observed from the compiled Admission CLI.
type AdmissionReply struct {
	Outcome, Stage                           string
	Link                                     string
	Request, Token, Response, Journal, Proof []byte
	Digest                                   [32]byte
	Slot                                     [32]byte
	Permission                               admission.Permission
	Repeated                                 bool
	Deadline                                 time.Time
	Bytes                                    uint64
}

// OpenAdmissionConsole runs and joins the compiled Admission console.
func OpenAdmissionConsole(t testing.TB, binary, operation, config string) (func(any) AdmissionReply, func()) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "admission", operation, "--config", config)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(errors.Join(err, in.Close()))
	}
	var log bytes.Buffer
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(errors.Join(err, in.Close(), out.Close()))
	}
	joined := false
	t.Cleanup(func() {
		if !joined {
			closeErr := in.Close()
			if errors.Is(closeErr, os.ErrClosed) {
				closeErr = nil
			}
			killErr := cmd.Process.Kill()
			if errors.Is(killErr, os.ErrProcessDone) {
				killErr = nil
			}
			waitErr := cmd.Wait()
			if err := errors.Join(closeErr, killErr); err != nil {
				t.Error("retire admission console", err)
			}
			if waitErr != nil {
				t.Log("admission console result during cleanup:", waitErr)
			}
		}
		if t.Failed() && log.Len() != 0 {
			t.Log("joined console diagnostics:", log.String())
		}
	})
	encode, decode := json.NewEncoder(in), json.NewDecoder(out)
	send := func(value any) AdmissionReply {
		t.Helper()
		if err := encode.Encode(value); err != nil {
			t.Fatal(err)
		}
		var reply AdmissionReply
		if err := decode.Decode(&reply); err != nil {
			t.Fatal(err)
		}
		return reply
	}
	close := func() {
		t.Helper()
		if joined {
			return
		}
		closeErr := in.Close()
		waitErr := cmd.Wait()
		joined = true
		if err := errors.Join(closeErr, waitErr); err != nil {
			t.Fatal(err, &log)
		}
	}
	return send, close
}
