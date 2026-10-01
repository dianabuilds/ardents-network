//go:build linux

package installation

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func observeInstalledSockets(ctx context.Context, endpoint worker.Properties) error {
	for _, key := range []string{"Requires", "After"} {
		property, present := endpoint[key]
		var names []string
		if !present || property.Type != "as" || json.Unmarshal(property.Data, &names) != nil ||
			!slices.Contains(names, "ardents-text-reader.socket") || !slices.Contains(names, "ardents-text-publisher.socket") {
			return errors.New("installed Endpoint socket dependencies differ")
		}
	}
	for _, role := range []string{"reader", "publisher"} {
		unit, socket, err := worker.ReadActivationSocketProperties(ctx, role)
		if err != nil {
			return err
		}
		if err := verifyInstalledSocket(unit, socket, role); err != nil {
			return err
		}
	}
	return nil
}

func verifyInstalledSocket(unit, socket worker.Properties, role string) error {
	if !propertyIs(unit, "ActiveState", "s", "active") || !propertyIs(unit, "SubState", "s", "listening") {
		return errors.New("installed activation socket is not listening")
	}
	return verifyBoundSocket(unit, socket, role)
}

func verifyBoundSocket(unit, socket worker.Properties, role string) error {
	if role != "reader" && role != "publisher" {
		return errors.New("installed activation role is invalid")
	}
	name := "ardents-text-" + role + ".socket"
	for key, want := range map[string]string{"Id": name, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + name} {
		if !propertyIs(unit, key, "s", want) {
			return errors.New("installed activation socket unit differs")
		}
	}
	if !propertyIs(unit, "DropInPaths", "as", []string{}) || !propertyIs(unit, "PartOf", "as", []string{"ardents-endpoint.service"}) ||
		!propertyIs(socket, "Accept", "b", true) || !propertyIs(socket, "SocketUser", "s", "ardents-endpoint") || !propertyIs(socket, "SocketGroup", "s", "ardents-endpoint") ||
		!propertyIs(socket, "SocketMode", "u", uint32(0600)) || !propertyIs(socket, "RemoveOnStop", "b", true) {
		return errors.New("installed activation socket ownership or lifetime differs")
	}
	return nil
}

func observeBoundSocketsForStop(ctx context.Context) error {
	for _, role := range []string{"reader", "publisher"} {
		unit, socket, err := worker.ReadActivationSocketProperties(ctx, role)
		if err != nil {
			return err
		}
		if err := verifyBoundSocket(unit, socket, role); err != nil {
			return err
		}
	}
	return nil
}
