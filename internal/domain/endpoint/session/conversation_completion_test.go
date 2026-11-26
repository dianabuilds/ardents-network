package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
)

type completionWriter struct {
	bytes.Buffer
	retired *bool
	t       *testing.T
}

func (w *completionWriter) Write(body []byte) (int, error) {
	if !*w.retired {
		w.t.Fatal("completion announced before original authority retired")
	}
	return w.Buffer.Write(body)
}
func TestHolderCompletionIncludesOriginalAuthorityCloseFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		retired := false
		holder := &localHolder{authority: funding.AdmissionAuthority{Close: func() error {
			retired = true
			if failed {
				return errors.New("PRIVATE cleanup")
			}
			return nil
		}}}
		out := &completionWriter{retired: &retired, t: t}
		code := holder.finishConversation(0, out)
		var report struct {
			Exit int `json:"exit_code"`
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		want := 0
		if failed {
			want = 1
		}
		if code != want || report.Exit != want || bytes.Contains(out.Bytes(), []byte("PRIVATE")) {
			t.Fatalf("close result erased: code=%d report=%s", code, out.String())
		}
	}
}
