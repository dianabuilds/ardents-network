package serviceconn

import (
	"testing"
	"time"
)

func TestRecoveryDeadlineStartsAtLastConnectionProgress(t *testing.T) {
	detected := time.Now()
	progress := detected.Add(-4 * time.Second)
	if got, want := recoveryEpisodeDeadline(progress, detected), progress.Add(recoveryLimit); !got.Equal(want) {
		t.Fatalf("deadline=%v want=%v", got, want)
	}
	if got, want := recoveryEpisodeDeadline(time.Time{}, detected), detected.Add(recoveryLimit); !got.Equal(want) {
		t.Fatalf("zero-progress deadline=%v want=%v", got, want)
	}
	terminal := progress.Add(recoveryLimit)
	if got, want := recoveryWorkDeadline(terminal), terminal.Add(-10*time.Millisecond); !got.Equal(want) {
		t.Fatalf("work deadline=%v want=%v", got, want)
	}
}
