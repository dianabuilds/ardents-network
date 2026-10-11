//go:build linux

package worker

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// value retains the D-Bus signature: an absent, null or differently
// typed hardening property never becomes a zero-value accepting observation.
type value = systemd.Value

type properties systemd.Properties

func validUnit(unit, role string) bool {
	if role != "reader" && role != "publisher" {
		return false
	}
	prefix := workerPrefix + "-" + role + "@"
	if !strings.HasPrefix(unit, prefix) || !strings.HasSuffix(unit, ".service") {
		return false
	}
	instance := strings.TrimSuffix(strings.TrimPrefix(unit, prefix), ".service")
	parts := strings.Split(instance, "-")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		value, err := strconv.ParseUint(part, 10, 32)
		if err != nil || strconv.FormatUint(value, 10) != part {
			return false
		}
	}
	return parts[1] != "0" && parts[2] != "0"
}

func readProperties(ctx context.Context, unit, role string) (properties, properties, error) {
	return readNamedProperties(ctx, unit, role, managerCall)
}

type managerQuery func(context.Context, string, string, string, string, ...string) (value, error)

func readCleanupProperties(ctx context.Context, instance Instance, query managerQuery) (properties, properties, error) {
	if ctx == nil || query == nil || !validUnit(instance.Name, instance.Role) || instance.Invocation == [16]byte{} {
		return nil, nil, errors.New("text worker cleanup identity is invalid")
	}
	// A name path may load a collected unit again without its invocation.
	// The original invocation path only resolves that retained invocation;
	// its disappearance remains unavailable, never a replacement Stop right.
	path := "/org/freedesktop/systemd1/unit/" + hex.EncodeToString(instance.Invocation[:])
	return readPropertiesAtPath(ctx, path, query)
}

func readNamedProperties(ctx context.Context, unit, role string, query managerQuery) (properties, properties, error) {
	if ctx == nil || query == nil || !validUnit(unit, role) {
		return nil, nil, errors.New("text worker unit identity is invalid")
	}
	var paths []string
	answer, err := query(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager", "GetUnit", "s", unit)
	if err != nil || answer.Type != "o" || json.Unmarshal(answer.Data, &paths) != nil || len(paths) != 1 || !strings.HasPrefix(paths[0], "/org/freedesktop/systemd1/unit/") {
		return nil, nil, errors.New("text worker system manager binding is unavailable")
	}
	return readPropertiesAtPath(ctx, paths[0], query)
}

func readPropertiesAtPath(ctx context.Context, path string, query managerQuery) (properties, properties, error) {
	var observations [2]properties
	for index, kind := range []string{"Unit", "Service"} {
		answer, err := query(ctx, path, "org.freedesktop.DBus.Properties", "GetAll", "s", "org.freedesktop.systemd1."+kind)
		var payload []properties
		if err != nil || answer.Type != "a{sv}" || json.Unmarshal(answer.Data, &payload) != nil || len(payload) != 1 || payload[0] == nil {
			return nil, nil, errors.New("text worker effective properties are unavailable")
		}
		observations[index] = payload[0]
	}
	return observations[0], observations[1], nil
}

// managerCall reaches only the installed system manager on the local system
// bus. It is read-only, bounded and noninteractive; no Application supplies an
// executable, environment, bus address, method, signature or object path.
func managerCall(ctx context.Context, path, iface, method, signature string, arguments ...string) (value, error) {
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	commandArguments := []string{"--system", "--json=short", "--no-pager", "call",
		"org.freedesktop.systemd1", path, iface, method, signature}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(bounded, "/usr/bin/busctl", commandArguments...)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	output := &managerOutput{}
	command.Stdout = output
	command.Stderr = io.Discard
	if err := runOriginalCommand(command); err != nil {
		// Keep a bounded, input-free reason while preserving the refusal.
		// The system bus response and stderr are never included in diagnostics.
		if ctx.Err() != nil {
			return value{}, errors.New("text worker system manager query was cancelled")
		}
		if bounded.Err() != nil {
			return value{}, errors.New("text worker system manager query timed out")
		}
		return value{}, errors.New("text worker system manager query failed")
	}
	var answer value
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&answer); err != nil {
		return value{}, errors.New("text worker system manager response is invalid")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return value{}, errors.New("text worker system manager response has trailing data")
	}
	return answer, nil
}

type managerOutput struct{ buffer bytes.Buffer }

func (output *managerOutput) Bytes() []byte { return output.buffer.Bytes() }

func (output *managerOutput) Write(body []byte) (int, error) {
	if len(body) > (512<<10)-output.buffer.Len() {
		return 0, errors.New("system manager response exceeds its bound")
	}
	return output.buffer.Write(body)
}

func (properties properties) exact(name, signature string, expected any) bool {
	value, ok := properties[name]
	if !ok || value.Type != signature {
		return false
	}
	want, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	var compact bytes.Buffer
	if json.Compact(&compact, value.Data) != nil {
		return false
	}
	return bytes.Equal(compact.Bytes(), want)
}
