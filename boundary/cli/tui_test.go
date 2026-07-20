package cli

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTUISectionNavigationWraps(t *testing.T) {
	if got := nextTUISection(tuiDiagnostics); got != tuiNode {
		t.Fatalf("nextTUISection() = %v, want %v", got, tuiNode)
	}
	if got := prevTUISection(tuiNode); got != tuiDiagnostics {
		t.Fatalf("prevTUISection() = %v, want %v", got, tuiDiagnostics)
	}
}

func TestTUIModelViewHighlightsActiveTabAndSnapshot(t *testing.T) {
	model := newTUIModel(context.Background(), &app{})
	model.active = tuiDiagnostics
	model.cache[tuiDiagnostics] = tuiSnapshot{
		Title: "Diagnostics",
		Lines: []string{"state: degraded", "pending operations: 2"},
	}

	text := model.View().Content
	for _, want := range []string{"[Diagnostics]", "state: degraded", "pending operations: 2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in view:\n%s", want, text)
		}
	}
}

func TestTUIModelArrowNavigationChangesActiveSection(t *testing.T) {
	model := newTUIModel(context.Background(), &app{})
	model.active = tuiNode
	model.loading = false

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	got := next.(tuiModel)
	if got.active != tuiNetwork {
		t.Fatalf("active = %v, want %v", got.active, tuiNetwork)
	}
}

func TestTUIActionForKeyFollowsDocumentedSectionsOnly(t *testing.T) {
	if action, ok := tuiActionForKey(tuiNode, "s"); !ok || action != tuiActionNodeStart {
		t.Fatalf("node start mapping = (%v, %v)", action, ok)
	}
	if _, ok := tuiActionForKey(tuiWorkloads, "c"); ok {
		t.Fatalf("workload section unexpectedly exposes action")
	}
	if _, ok := tuiActionForKey(tuiDiagnostics, "c"); ok {
		t.Fatalf("diagnostics unexpectedly exposes action")
	}
}
