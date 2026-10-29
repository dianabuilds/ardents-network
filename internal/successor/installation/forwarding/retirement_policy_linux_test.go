package forwarding

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

func TestForwardingStopFailureKeepsCausesPrivate(t *testing.T) {
	private := errors.New("private path and manager observation")
	for phase := 0; phase < 256; phase++ {
		failure := &stopFailure{phase: stopPhase(phase), cause: errors.Join(ErrBinding, context.Canceled, private)}
		if strings.Contains(failure.Error(), private.Error()) || !errors.Is(failure, private) || !errors.Is(failure, ErrBinding) || !errors.Is(failure, context.Canceled) {
			t.Fatal("diagnostic lost retained causes or printed private details", phase)
		}
		if phase > int(stopRetirement) && failure.Error() != "forwarding stop: internal refused" {
			t.Fatal("unknown stage escaped the finite vocabulary", phase)
		}
	}
	if (&stopFailure{phase: stopExecutionClock, cause: private}).Error() != "forwarding stop: execution-clock refused" {
		t.Fatal("execution clock failure was not distinguishable")
	}
}

func TestForwardingReplacementStoppedDoesNotRebindPredecessorExecution(t *testing.T) {
	previous, selected := replacementSelectionVector(t)
	id := [16]byte{9}
	a := startArchive{Generation: previous.binding.Generation, BindingSHA256: digest(previous.raw), InvocationID: hex.EncodeToString(id[:]), Manager: ":1.17", PID: 7317, StartedRealtime: 30, StartedMonotonic: 40}
	r := retirementResult{Generation: a.Generation, BindingSHA256: a.BindingSHA256, Attempt: 0, InvocationID: a.InvocationID, Manager: a.Manager, PID: a.PID, StartedRealtime: 30, StartedMonotonic: 40, ExitedRealtime: 50, ExitedMonotonic: 60, Code: 1}
	u, s := forwardingLoadedVector(t)
	set := func(p systemd.Properties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		p[name] = systemd.Value{Type: signature, Data: raw}
	}
	set(u, "TriggeredBy", "as", []string{})
	set(s, "ControlPID", "u", uint32(0))
	set(s, "NRestarts", "u", uint32(0))
	set(s, "DynamicUser", "b", false)
	set(s, "StandardInput", "s", "null")
	request := selected.binding.Request
	set(s, "ReadWritePaths", "as", []string{request.StateRoot, request.ParticipationRoot, request.SpendRoot, request.HostingRoot})
	program := filepath.Join(request.InstallationRoot, "generations", selected.binding.Generation, "ardents-node-linux-amd64")
	set(s, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "forwarding", "start-installed", filepath.Join(request.InstallationRoot, "binding.json")}, []string{"no-env-expand"}, uint64(0), uint64(0), uint64(0), uint64(0), uint32(0), int32(0), int32(0)}})
	retirementSHA := selected.replacement.RetirementSHA256
	check := func() error { return verifyForwardingReplacementStopped(u, s, selected, a, 0, id, r, retirementSHA) }
	if err := check(); err != nil {
		t.Fatal("new zero-execution configuration with separate predecessor exit", err)
	}
	if verifyForwardingRestartStopped(u, s, request, selected.binding.Generation, a, id, r.execution()) == nil {
		t.Fatal("old execution rebound by same-generation policy")
	}
	for _, fault := range []string{"generation", "binding", "attempt", "retirement", "manager", "invocation", "exit", "signal", "missing-retirement"} {
		t.Run(fault, func(t *testing.T) {
			badArchive, badResult, badNumber, badSHA := a, r, attemptNumber(0), retirementSHA
			switch fault {
			case "generation":
				badArchive.Generation = selected.binding.Generation
			case "binding":
				badArchive.BindingSHA256 = digest([]byte("foreign"))
			case "attempt":
				badNumber = 1
			case "retirement":
				badSHA = digest([]byte("foreign"))
			case "manager":
				badResult.Manager = ":1.18"
			case "invocation":
				badResult.InvocationID = strings.Repeat("0", 32)
			case "exit":
				badResult.PID++
			case "signal":
				badResult.Code = 2
			case "missing-retirement":
				badResult = retirementResult{}
			}
			if verifyForwardingReplacementStopped(u, s, selected, badArchive, badNumber, id, badResult, badSHA) == nil {
				t.Fatal("foreign predecessor comparison accepted")
			}
		})
	}
	for _, change := range []struct {
		p               systemd.Properties
		name, signature string
		value           any
	}{
		{u, "InvocationID", "ay", id}, {u, "TriggeredBy", "as", []string{"foreign.socket"}},
		{s, "ControlPID", "u", uint32(1)}, {s, "NRestarts", "u", uint32(1)}, {s, "DynamicUser", "b", true},
		{s, "ExecMainPID", "u", a.PID}, {s, "ExecMainStartTimestamp", "t", a.StartedRealtime}, {s, "StandardInput", "s", "socket"},
	} {
		original := change.p[change.name]
		set(change.p, change.name, change.signature, change.value)
		if check() == nil {
			t.Fatal("loaded candidate hid nonzero/foreign native state", change.name)
		}
		change.p[change.name] = original
	}
	if a.Generation != previous.binding.Generation || r.Generation != a.Generation {
		t.Fatal("comparison changed original execution")
	}
}

// These are detached property/byte comparisons. No vector enters a live Stop,
// Startup, process, scope or manager, and none supplies joined completion.
func TestForwardingExitedPolicyRequiresOriginalCleanMainExecution(t *testing.T) {
	r := Request{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation", SpendRoot: "/var/lib/ardents-spend", HostingRoot: "/var/lib/ardents-hosting"}
	g, id := strings.Repeat("a", 64), [16]byte{9}
	a := startArchive{PID: 7317, InvocationID: hex.EncodeToString(id[:]), Generation: g, StartedRealtime: 30, StartedMonotonic: 40}
	u, s := forwardingLoadedVector(t)
	set := func(p systemd.Properties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		p[name] = systemd.Value{Type: signature, Data: raw}
	}
	set(u, "TriggeredBy", "as", []string{})
	set(u, "InvocationID", "ay", id)
	set(s, "ExecMainPID", "u", a.PID)
	for _, name := range []string{"ControlPID", "NRestarts"} {
		set(s, name, "u", uint32(0))
	}
	set(s, "DynamicUser", "b", false)
	set(s, "StandardInput", "s", "null")
	for name, value := range map[string]uint64{"ExecMainStartTimestamp": 30, "ExecMainStartTimestampMonotonic": 40, "ExecMainExitTimestamp": 50, "ExecMainExitTimestampMonotonic": 60} {
		set(s, name, "t", value)
	}
	set(s, "ExecMainCode", "i", int32(1))
	program := "/opt/ardents/generations/" + g + "/ardents-node-linux-amd64"
	set(s, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "forwarding", "start-installed", "/opt/ardents/binding.json"}, []string{"no-env-expand"}, uint64(30), uint64(40), uint64(50), uint64(60), a.PID, int32(1), int32(0)}})
	check := func() error { _, err := verifyForwardingExited(u, s, r, g, a, id); return err }
	if err := check(); err != nil {
		t.Fatal("complete clean exit grammar", err)
	}
	for _, p := range []systemd.Properties{u, s} {
		for name, original := range p {
			delete(p, name)
			if check() == nil {
				t.Fatal("missing property", name)
			}
			p[name] = systemd.Value{Type: "v", Data: original.Data}
			if check() == nil {
				t.Fatal("wrong signature", name)
			}
			p[name] = original
		}
	}
	for _, change := range []struct {
		p         systemd.Properties
		name, sig string
		value     any
	}{
		{u, "InvocationID", "ay", [16]byte{10}}, {u, "ActiveState", "s", "failed"}, {u, "TriggeredBy", "as", []string{"foreign.socket"}},
		{s, "MainPID", "u", uint32(7317)}, {s, "ExecMainPID", "u", uint32(7318)}, {s, "ExecMainCode", "i", int32(2)}, {s, "ExecMainStatus", "i", int32(15)},
		{s, "ExecMainStartTimestamp", "t", uint64(29)}, {s, "ExecMainExitTimestamp", "t", uint64(29)}, {s, "ExecMainExitTimestampMonotonic", "t", uint64(39)},
		{s, "ControlGroup", "s", "/foreign"}, {s, "NRestarts", "u", uint32(1)}, {s, "DynamicUser", "b", true}, {s, "MemoryMax", "t", uint64(1073741825)},
	} {
		old := change.p[change.name]
		set(change.p, change.name, change.sig, change.value)
		if check() == nil {
			t.Fatal("changed stopped execution admitted", change.name)
		}
		change.p[change.name] = old
	}
	set(u, "InvocationID", "ay", []any{})
	set(s, "ControlGroup", "s", "/system.slice/ardents-forwarding.service")
	if err := check(); err != nil {
		t.Fatal("collected invocation with retained original execution", err)
	}
	if verifyForwardingStopped(u, s, r, g) == nil {
		t.Fatal("retired execution generalized initial stopped state")
	}
}

func TestForwardingRetirementResultBindsOriginalFrameAndMainClocks(t *testing.T) {
	id, challenge := [16]byte{9}, [32]byte{7}
	b := binding{Generation: strings.Repeat("a", 64)}
	bh, sh := strings.Repeat("b", 64), strings.Repeat("c", 64)
	a := startArchive{Generation: b.Generation, BindingSHA256: bh, PID: 7317, InvocationID: hex.EncodeToString(id[:]), Manager: ":1.17", StartedRealtime: 30, StartedMonotonic: 40}
	start, err := completion.Encode(id, b.Generation, bh)
	if err != nil {
		t.Fatal(err)
	}
	frame := retirementFrame(start, challenge)
	r := retirementResult{Schema: "ardents-forwarding-retirement-v1", Attempt: 0, Generation: b.Generation, BindingSHA256: bh, StartSHA256: sh, InvocationID: a.InvocationID, PID: a.PID, Manager: a.Manager, Challenge: hex.EncodeToString(challenge[:]), CompletionSHA256: digest(frame[:]), StartedRealtime: 30, StartedMonotonic: 40, ExitedRealtime: 50, ExitedMonotonic: 60, Code: 1, Status: 0}
	encode := func(value retirementResult) []byte {
		raw, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		return append(raw, '\n')
	}
	decode := func(raw []byte) error { _, e := decodeRetirementResult(raw, 0, b, bh, sh, a, id); return e }
	if err := decode(encode(r)); err != nil {
		t.Fatal("exact independent retirement result", err)
	}
	for name, mutate := range map[string]func(*retirementResult){
		"purpose": func(v *retirementResult) { v.Schema = "ardents-source-retirement-v1" }, "attempt": func(v *retirementResult) { v.Attempt = 1 },
		"generation": func(v *retirementResult) { v.Generation = sh }, "binding": func(v *retirementResult) { v.BindingSHA256 = sh }, "start": func(v *retirementResult) { v.StartSHA256 = bh },
		"invocation": func(v *retirementResult) { v.InvocationID = strings.Repeat("0", 32) }, "pid": func(v *retirementResult) { v.PID++ }, "manager": func(v *retirementResult) { v.Manager = ":1.18" },
		"challenge": func(v *retirementResult) { v.Challenge = strings.Repeat("0", 64) }, "replayed frame": func(v *retirementResult) { v.CompletionSHA256 = sh },
		"main start": func(v *retirementResult) { v.StartedRealtime-- }, "main mono": func(v *retirementResult) { v.StartedMonotonic-- },
		"exit clock": func(v *retirementResult) { v.ExitedRealtime = 29 }, "exit mono": func(v *retirementResult) { v.ExitedMonotonic = 39 }, "signal": func(v *retirementResult) { v.Code = 2 }, "status": func(v *retirementResult) { v.Status = 1 },
	} {
		changed := r
		mutate(&changed)
		if decode(encode(changed)) == nil {
			t.Fatal("substitution admitted", name)
		}
	}
	if decode(append(encode(r), '\n')) == nil || decode(encode(r)[:len(encode(r))-1]) == nil {
		t.Fatal("noncanonical framing admitted")
	}
}

func TestForwardingRestartStoppedComparisonDoesNotInventCollectedRetirement(t *testing.T) {
	r := Request{InstallationRoot: "/opt/ardents", StateRoot: "/var/lib/ardents-state", ParticipationRoot: "/var/lib/ardents-participation", SpendRoot: "/var/lib/ardents-spend", HostingRoot: "/var/lib/ardents-hosting"}
	g, id := strings.Repeat("a", 64), [16]byte{9}
	a := startArchive{Generation: g, InvocationID: hex.EncodeToString(id[:]), PID: 7317, StartedRealtime: 30, StartedMonotonic: 40}
	exit := forwardingExecution{30, 40, 50, 60, 7317, 1, 0}
	u, s := forwardingLoadedVector(t)
	set := func(p systemd.Properties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		p[name] = systemd.Value{Type: signature, Data: raw}
	}
	set(u, "TriggeredBy", "as", []string{})
	set(s, "ControlPID", "u", uint32(0))
	set(s, "NRestarts", "u", uint32(0))
	set(s, "DynamicUser", "b", false)
	set(s, "StandardInput", "s", "null")
	if err := verifyForwardingRestartStopped(u, s, r, g, a, id, exit); err != nil {
		t.Fatal("collected native grammar with separately matched detached exit", err)
	}
	if verifyForwardingRestartStopped(u, s, r, g, a, id, forwardingExecution{}) == nil {
		t.Fatal("collected zeros invented missing original exit")
	}
	for _, change := range []struct {
		p         systemd.Properties
		name, sig string
		value     any
	}{
		{u, "InvocationID", "ay", [16]byte{10}}, {u, "TriggeredBy", "as", []string{"foreign.socket"}},
		{s, "ControlPID", "u", uint32(1)}, {s, "NRestarts", "u", uint32(1)}, {s, "DynamicUser", "b", true}, {s, "StandardInput", "s", "socket"}, {s, "MemoryMax", "t", uint64(1073741825)},
	} {
		old := change.p[change.name]
		set(change.p, change.name, change.sig, change.value)
		if verifyForwardingRestartStopped(u, s, r, g, a, id, exit) == nil {
			t.Fatal("collected unit hid native change", change.name)
		}
		change.p[change.name] = old
	}
}
