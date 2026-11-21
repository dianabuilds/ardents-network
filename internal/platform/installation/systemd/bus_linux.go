package systemd

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

func Endpoint(ctx context.Context) (Properties, Properties, error) {
	// A stopped unit may be collected between short-lived bus clients. GetAll
	// on its fixed object path loads its configuration within that same method
	// dispatch; GetUnit would require it to remain loaded from an earlier call.
	// The typed Unit Id, fragment and stopped state still have to match below.
	const object = "/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice"
	return properties(ctx, object, "Service")
}

// No public input selects a binary, system bus, method or environment. The only
// callers request the fixed Endpoint and its two activation socket observations.
func call(ctx context.Context, object, iface, method, argument string) (Value, error) {
	if ctx == nil {
		return Value{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Value{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/busctl", "--system", "--json=short", "--no-pager", "call", "org.freedesktop.systemd1", object, iface, method, "s", argument)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := errors.Join(runOriginalManagerCommand(command), bounded.Err(), ctx.Err()); err != nil {
		return Value{}, err
	}
	if diagnostic.body.Len() != 0 {
		return Value{}, ErrUnavailable
	}
	var value Value
	if Decode([]byte(strings.TrimSpace(output.body.String())), &value) != nil || value.Type == "" || len(value.Data) == 0 {
		return Value{}, ErrObservation
	}
	return value, ctx.Err()
}

// Activation returns typed facts for one fixed socket object. It accepts no
// bus, binary, arbitrary object, method or environment selection from a caller.
func Activation(ctx context.Context, role string) (Properties, Properties, error) {
	if role != "reader" && role != "publisher" {
		return nil, nil, ErrInput
	}
	object := "/org/freedesktop/systemd1/unit/ardents_2dtext_2d" + role + "_2esocket"
	return properties(ctx, object, "Socket")
}

func properties(ctx context.Context, object, kind string) (Properties, Properties, error) {
	var observations [2]Properties
	for index, iface := range []string{"Unit", kind} {
		answer, err := call(ctx, object, "org.freedesktop.DBus.Properties", "GetAll", "org.freedesktop.systemd1."+iface)
		var payload []Properties
		if err != nil || answer.Type != "a{sv}" || Decode(answer.Data, &payload) != nil || len(payload) != 1 || payload[0] == nil {
			return nil, nil, errors.Join(ErrObservation, err)
		}
		observations[index] = payload[0]
	}
	return observations[0], observations[1], ctx.Err()
}

type boundedManagerOutput struct{ body strings.Builder }

func (output *boundedManagerOutput) Write(body []byte) (int, error) {
	if output.body.Len()+len(body) > 64<<10 {
		return 0, ErrUnavailable
	}
	return output.body.Write(body)
}
