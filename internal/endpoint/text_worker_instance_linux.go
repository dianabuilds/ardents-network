//go:build linux

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
)

// textWorkerListing is a system-manager observation, not a launch receipt.
// Keep failed and activating units too: filtering to active units would let a
// delayed previous activation masquerade as the next job's new worker.
type textWorkerListing map[string]textWorkerUnitState

type textWorkerUnitState struct {
	active string
	// pendingStart is proven by the same manager tuple as inactive/dead.
	// It permits waiting for this unit, never readiness or a Grant.
	pendingStart bool
}

func listTextWorkerInstances(ctx context.Context, role string) (textWorkerListing, error) {
	if role != "reader" && role != "publisher" || os.Getpid() <= 0 || os.Geteuid() <= 0 {
		return nil, errors.New("text worker activation owner is unavailable")
	}
	suffix := "-" + strconv.Itoa(os.Getpid()) + "-" + strconv.Itoa(os.Geteuid()) + ".service"
	answer, err := textManagerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager",
		"ListUnitsByPatterns", "asas", "0", "1", "ardents-text-"+role+"@*"+suffix)
	if err != nil {
		return nil, err
	}
	return decodeTextWorkerInstances(answer, role, suffix)
}

func decodeTextWorkerInstances(answer textManagerValue, role, suffix string) (textWorkerListing, error) {
	var payload [][][]json.RawMessage
	if answer.Type != "a(ssssssouso)" || json.Unmarshal(answer.Data, &payload) != nil || len(payload) != 1 || payload[0] == nil || len(payload[0]) > 128 {
		return nil, errors.New("text worker activation inventory is unavailable")
	}
	found := make(textWorkerListing, len(payload[0]))
	for _, entry := range payload[0] {
		var name, state, loaded, substate, jobType, jobPath string
		var jobID uint32
		if len(entry) != 10 || json.Unmarshal(entry[0], &name) != nil || json.Unmarshal(entry[3], &state) != nil ||
			!textWorkerUnit(name, role) || !strings.HasSuffix(name, suffix) || state == "" {
			return nil, errors.New("text worker activation inventory is invalid")
		}
		if json.Unmarshal(entry[2], &loaded) != nil || loaded == "" ||
			json.Unmarshal(entry[4], &substate) != nil || substate == "" ||
			json.Unmarshal(entry[7], &jobID) != nil || strings.TrimSpace(string(entry[7])) == "null" ||
			json.Unmarshal(entry[8], &jobType) != nil || strings.TrimSpace(string(entry[8])) == "null" ||
			json.Unmarshal(entry[9], &jobPath) != nil ||
			(jobID == 0 && (jobType != "" || jobPath != "/")) ||
			(jobID != 0 && (jobType == "" || jobPath != "/org/freedesktop/systemd1/job/"+strconv.FormatUint(uint64(jobID), 10))) {
			return nil, errors.New("text worker activation job is unavailable")
		}
		if _, duplicate := found[name]; duplicate {
			return nil, errors.New("text worker activation inventory is ambiguous")
		}
		found[name] = textWorkerUnitState{active: state, pendingStart: loaded == "loaded" &&
			state == "inactive" && substate == "dead" && jobID != 0 && jobType == "start"}
	}
	return found, nil
}

// newTextWorkerInstance only selects a newly observed unit. Its caller owns a
// serial activation gate from the baseline through joined cleanup on failure;
// a timed-out activation must never release that gate for a replacement job.
func newTextWorkerInstance(before, after textWorkerListing, candidate string) (string, error) {
	if before == nil || after == nil {
		return "", errors.New("text worker activation inventory is absent")
	}
	var selected string
	for name, state := range after {
		if _, old := before[name]; old {
			continue
		}
		if selected != "" {
			return "", errors.New("text worker activation inventory contains multiple new units")
		}
		if state.active != "active" && state.active != "activating" && !state.pendingStart {
			return "", errors.New("text worker activation has no active or pending start")
		}
		selected = name
	}
	if candidate != "" && selected != candidate {
		return "", errors.New("text worker activation candidate disappeared or changed")
	}
	return selected, nil
}

// textWorkerInstance binds manager-owned identity to one live activation.
// It still grants nothing: exact socket credentials, readiness, current job,
// artifact and a cleanup owner must all be verified before a Principal exists.
type textWorkerInstance struct {
	name       string
	role       string
	cgroup     string
	pid        uint32
	uid        uint32
	invocation [16]byte
}

func observeTextWorkerInstance(ctx context.Context, name, role string) (textWorkerInstance, error) {
	unit, service, err := readTextWorkerProperties(ctx, name, role)
	if err != nil {
		return textWorkerInstance{}, err
	}
	observed := textWorkerInstance{name: name, role: role}
	pid, pidOK := service["MainPID"]
	group, groupOK := service["ControlGroup"]
	id, idOK := unit["InvocationID"]
	if !pidOK || pid.Type != "u" || json.Unmarshal(pid.Data, &observed.pid) != nil ||
		!groupOK || group.Type != "s" || json.Unmarshal(group.Data, &observed.cgroup) != nil ||
		!idOK || !decodeTextWorkerInvocation(id, &observed.invocation) {
		return textWorkerInstance{}, errors.New("text worker invocation is unavailable")
	}
	if observed.invocation == [16]byte{} || !unit.exact("TriggeredBy", "as", []string{"ardents-text-" + role + ".socket"}) {
		return textWorkerInstance{}, errors.New("text worker socket activation is unavailable")
	}
	if err := verifyTextWorkerProperties(unit, service, name, role, observed.cgroup, observed.pid); err != nil {
		return textWorkerInstance{}, err
	}
	userPrefix := "ardtxt-r-"
	if role == "publisher" {
		userPrefix = "ardtxt-p-"
	}
	user := userPrefix + strings.TrimSuffix(strings.TrimPrefix(name, "ardents-text-"+role+"@"), ".service")
	answer, err := textManagerCall(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "LookupDynamicUserByName", "s", user)
	var users []uint32
	if err != nil || answer.Type != "u" || json.Unmarshal(answer.Data, &users) != nil || len(users) != 1 || users[0] < 61184 || users[0] > 65519 || users[0] == uint32(os.Geteuid()) {
		return textWorkerInstance{}, errors.New("text worker DynamicUser is unavailable")
	}
	observed.uid = users[0]
	return observed, nil
}

// Unlike []byte decoding, this accepts only a complete numeric D-Bus ay
// observation. Base64 strings and null elements are not known identity bytes.
func decodeTextWorkerInvocation(value textManagerValue, destination *[16]byte) bool {
	if value.Type != "ay" || destination == nil {
		return false
	}
	var parts []json.RawMessage
	if json.Unmarshal(value.Data, &parts) != nil || len(parts) != len(destination) {
		return false
	}
	var decoded [16]byte
	for index, part := range parts {
		parsed, err := strconv.ParseUint(strings.TrimSpace(string(part)), 10, 8)
		if err != nil {
			return false
		}
		decoded[index] = byte(parsed)
	}
	if decoded == [16]byte{} {
		return false
	}
	*destination = decoded
	return true
}
