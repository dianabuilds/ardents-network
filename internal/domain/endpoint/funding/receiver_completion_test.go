package funding

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

type completionWriter struct {
	bytes.Buffer
	retired *bool
	t       *testing.T
}

func (w *completionWriter) Write(body []byte) (int, error) {
	if !*w.retired {
		w.t.Fatal("receiver completion announced before authority retired")
	}
	return w.Buffer.Write(body)
}
func TestReceiverCompletionIncludesOriginalAuthorityCloseFailure(t *testing.T) {
	retired := false
	receiver := &localReceiver{authority: AdmissionAuthority{Close: func() error { retired = true; return errors.New("PRIVATE cleanup") }}}
	out := &completionWriter{retired: &retired, t: t}
	code := receiver.finishConversation(0, out)
	var report struct {
		Exit int `json:"exit_code"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if code != 1 || report.Exit != 1 || bytes.Contains(out.Bytes(), []byte("PRIVATE")) {
		t.Fatalf("close result erased: %d %s", code, out.String())
	}
}
