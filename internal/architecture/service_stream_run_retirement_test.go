package architecture

import (
	"strings"
	"testing"
)

// ADR-0097 retired the exact-count Stream.Run pump: the accepted Linux text
// Endpoint runtime drives RunBounded, and no accepted C0 command started the
// standalone exact-workload service candidate.
func TestExactCountStreamRunPumpIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for file, forbidden := range map[string][]string{
		"internal/service/connection/stream_lifecycle.go": {"func (stream *Stream) Run("},
		"internal/service/connection/stream_send.go": {
			"func (stream *Stream) sendApplication(",
			"func (stream *Stream) sendAcknowledgements(",
			"func (stream *Stream) sendTerminal(",
		},
		"internal/service/connection/stream_receive.go": {"func (stream *Stream) receiveApplication("},
	} {
		content := string(readProjectFile(t, root, file))
		for _, symbol := range forbidden {
			if strings.Contains(content, symbol) {
				t.Errorf("%s still contains retired exact-count pump member %q", file, symbol)
			}
		}
	}
}

func TestStreamRunRetirementPreservesBoundedSuccessor(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, retained := range []struct {
		path        string
		declaration string
	}{
		{"internal/service/connection/stream_bounded.go", "func (stream *Stream) RunBounded(sendLimit, receiveLimit uint32) (Outcome, error)"},
		{"internal/service/connection/stream_bounded.go", "func (stream *Stream) sendApplicationBounded("},
		{"internal/service/connection/stream_bounded.go", "func (stream *Stream) receiveApplicationBounded("},
		{"internal/service/connection/stream_bounded.go", "func (stream *Stream) sendBoundedAcknowledgements("},
		{"internal/service/connection/stream_lifecycle.go", "func (stream *Stream) establishInitialAttachment("},
		{"internal/service/connection/stream_lifecycle.go", "func (stream *Stream) watchNameOrigin("},
		{"internal/service/connection/stream_send.go", "func (stream *Stream) flushAvailable("},
		{"internal/service/connection/stream_send.go", "func (stream *Stream) writeRecord("},
		{"internal/service/connection/stream_receive.go", "func (stream *Stream) acceptData("},
		{"internal/service/connection/stream_receive.go", "func (stream *Stream) acknowledgeLocked("},
	} {
		content := string(readProjectFile(t, root, retained.path))
		if !strings.Contains(content, retained.declaration) {
			t.Errorf("%s lost retained declaration %q", retained.path, retained.declaration)
		}
	}
	endpoint := string(readProjectFile(t, root, "internal/endpoint/text_service_stream.go"))
	if !strings.Contains(endpoint, "stream.RunBounded(") {
		t.Error("text Endpoint runtime lost its live RunBounded wiring")
	}
}
