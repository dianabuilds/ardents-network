package processfixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
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
	Recipient                                AdmissionRecipient
	Registration                             AdmissionRegistration
	Stock                                    AdmissionStock
}

// AdmissionStock is the public holder status returned by the console.
type AdmissionStock struct {
	Accepted, Pending, Busy, Closed bool
	Remaining                       [3]uint32
	BootstrapRemaining              uint8
}

// AdmissionRecipient is the public Rendezvous choice returned by the holder.
type AdmissionRecipient struct {
	Node       [32]byte
	Generation uint64
	NotAfter   time.Time `json:"not_after"`
}

// AdmissionRegistration is the public binding returned by a completed REGISTER.
type AdmissionRegistration struct {
	Network, Profile, Node, Slot [32]byte
	Revision                     uint64
	Created, Expiry              time.Time
	Acknowledgement              [32]byte
}

// OpenAdmissionConsole runs and joins the compiled Admission console.
func OpenAdmissionConsole(t testing.TB, binary, operation, config string) (func(any) AdmissionReply, func()) {
	t.Helper()
	send, finish := OpenAdmissionConsoleResult(t, binary, operation, config)
	return send, func() {
		t.Helper()
		if err := finish(); err != nil {
			t.Fatal("admission console result:", err)
		}
	}
}

// OpenAdmissionConsoleResult returns the actual joined process result to tests
// that must distinguish a retained failure from a successful EOF.
func OpenAdmissionConsoleResult(t testing.TB, binary, operation, config string) (func(any) AdmissionReply, func() error) {
	send, finish, _ := openAdmissionConsole(t, binary, operation, config, nil)
	return send, finish
}

func openAdmissionConsole(t testing.TB, binary, operation, config string, signal func(*exec.Cmd) error) (func(any) AdmissionReply, func() error, func() error) {
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
	close := func() error {
		t.Helper()
		if joined {
			return nil
		}
		closeErr := in.Close()
		waitErr := cmd.Wait()
		joined = true
		return errors.Join(closeErr, waitErr)
	}
	interrupt := func() error {
		t.Helper()
		if signal == nil {
			return errors.New("console interruption unavailable")
		}
		if err := signal(cmd); err != nil {
			return err
		}
		waitErr := cmd.Wait()
		joined = true
		closeErr := in.Close()
		if errors.Is(closeErr, os.ErrClosed) {
			closeErr = nil
		}
		return errors.Join(waitErr, closeErr)
	}
	return send, close, interrupt
}

// HolderCommand is the test request sent to the compiled holder console.
type HolderCommand struct {
	Operation    string               `json:"operation"`
	Maxima       [3]uint32            `json:"maxima,omitzero"`
	Digest       [32]byte             `json:"digest,omitzero"`
	Payload      []byte               `json:"payload,omitzero"`
	Intent       stock.IssuanceIntent `json:"intent,omitzero"`
	Presentation stock.Presentation   `json:"presentation,omitzero"`
	Class        uint8                `json:"class,omitzero"`
	Receivers    [][32]byte           `json:"receivers,omitzero"`
	Revision     uint64               `json:"revision,omitzero"`
	Target       [32]byte             `json:"target,omitzero"`
}

// RunAdmissionCommand runs one Admission command through completion and decodes its reply.
func RunAdmissionCommand(t testing.TB, binary, operation string, config any) AdmissionReply {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "admission", operation, "--config", WriteConfig(t, config))
	var output, log bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &log
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %v %s %s", operation, err, &output, &log)
	}
	var reply AdmissionReply
	if err := json.Unmarshal(output.Bytes(), &reply); err != nil {
		t.Fatal(err, &output)
	}
	return reply
}

// OpenConfiguredAdmissionConsole writes the input for a bounded Admission console.
func OpenConfiguredAdmissionConsole(t testing.TB, binary, operation string, config any) (func(any) AdmissionReply, func()) {
	t.Helper()
	return OpenAdmissionConsole(t, binary, operation, WriteConfig(t, config))
}
