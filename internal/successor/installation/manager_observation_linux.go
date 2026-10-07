package installation

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

func (owned *initialPreparation) observeStoppedInstallation(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := observeStoppedManager(ctx); err != nil {
		return err
	}
	version, err := observeSystemManagerVersion(ctx)
	if err != nil {
		return err
	}
	unit, service, err := readEndpointManagerProperties(ctx)
	if err != nil {
		return err
	}
	if err := verifyStoppedEndpointProperties(version, unit, service, *request.declared, owned.stage.selected.GenerationDigest); err != nil {
		return err
	}
	// Unit and Service are separate manager observations. Reobserve stopped
	// identity after reading both; no active/queued process gets a late handoff.
	if err := observeStoppedManager(ctx); err != nil {
		return err
	}
	return owned.observe(ctx, request)
}

func readEndpointManagerProperties(ctx context.Context) (managerProperties, managerProperties, error) {
	// A stopped unit may be collected between short-lived bus clients. GetAll
	// on its fixed object path loads its configuration within that same method
	// dispatch; GetUnit would require it to remain loaded from an earlier call.
	// The typed Unit Id, fragment and stopped state still have to match below.
	const object = "/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice"
	var observations [2]managerProperties
	for index, kind := range []string{"Unit", "Service"} {
		answer, err := callInstallationManager(ctx, object, "org.freedesktop.DBus.Properties", "GetAll", "org.freedesktop.systemd1."+kind)
		var payload []managerProperties
		if err != nil || answer.Type != "a{sv}" || decodeManagerJSON(answer.Data, &payload) != nil || len(payload) != 1 || payload[0] == nil {
			return nil, nil, errors.Join(ErrBinding, err)
		}
		observations[index] = payload[0]
	}
	return observations[0], observations[1], ctx.Err()
}

// No public input selects a binary, system bus, method or environment. The only
// callers request the exact installed Endpoint Unit and Service observations.
func callInstallationManager(ctx context.Context, object, iface, method, argument string) (managerValue, error) {
	if err := ctx.Err(); err != nil {
		return managerValue{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/bin/busctl", "--system", "--json=short", "--no-pager", "call", "org.freedesktop.systemd1", object, iface, method, "s", argument)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := errors.Join(command.Run(), bounded.Err(), ctx.Err()); err != nil {
		return managerValue{}, err
	}
	if diagnostic.body.Len() != 0 {
		return managerValue{}, ErrNativeUnavailable
	}
	var value managerValue
	if decodeManagerJSON([]byte(strings.TrimSpace(output.body.String())), &value) != nil || value.Type == "" || len(value.Data) == 0 {
		return managerValue{}, ErrBinding
	}
	return value, ctx.Err()
}
