package systemd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// StartupObservation contains detached fresh facts, not startup or Release
// authority. Both rounds come from one original manager and bus connection.
type StartupObservation struct {
	Version, Manager string
	Rounds           [2]StartupRound
}

type StartupRound struct {
	EndpointUnit, EndpointService  Properties
	ReaderUnit, ReaderSocket       Properties
	PublisherUnit, PublisherSocket Properties
}

// ObserveStartup performs the same fixed Version and two rounds of six GetAll
// queries as the subprocess composition. Every invocation opens fresh physical
// custody, pins the unique manager, checks it again, then joins the connection.
// No public input selects a bus, object, method, auth identity or reusable cache.
func ObserveStartup(ctx context.Context) (result StartupObservation, returnedErr error) {
	if ctx == nil {
		return result, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	acquire, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	socket, err := (&net.Dialer{}).DialContext(acquire, "unix", "/run/dbus/system_bus_socket")
	if err != nil {
		return result, errors.Join(ErrUnavailable, err)
	}
	unix, ok := socket.(*net.UnixConn)
	if !ok {
		return result, errors.Join(ErrUnavailable, socket.Close())
	}
	transport := &referenceTransport{socket: unix, properties: true}
	var bus *dbus.Conn
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = transport.Close(); close(interrupted) })
	defer func() {
		if !stop() {
			<-interrupted
		}
		if bus != nil {
			returnedErr = errors.Join(returnedErr, bus.Close())
		}
		returnedErr = errors.Join(returnedErr, transport.Close(), ctx.Err())
		if returnedErr != nil {
			result = StartupObservation{}
		}
	}()
	raw, err := unix.SyscallConn()
	if err != nil {
		return result, err
	}
	var peer *syscall.Ucred
	var nativeErr error
	if err = raw.Control(func(fd uintptr) {
		peer, nativeErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || nativeErr != nil || peer == nil || peer.Uid != 0 || peer.Pid <= 0 {
		return result, errors.Join(ErrObservation, err, nativeErr)
	}
	end, _ := acquire.Deadline()
	if err := unix.SetDeadline(end); err != nil {
		return result, err
	}
	bus, err = dbus.NewConn(transport, dbus.WithContext(context.WithoutCancel(ctx)))
	if err != nil {
		return result, err
	}
	if err := bus.Auth([]dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Geteuid()))}); err != nil {
		return result, err
	}
	if err := bus.Hello(); err != nil {
		return result, err
	}
	client := startupBus{bus: bus, transport: transport}
	if err := client.owner(ctx, &result.Manager); err != nil {
		return result, err
	}
	if !strings.HasPrefix(result.Manager, ":") || len(result.Manager) > 255 {
		return result, ErrObservation
	}
	client.manager = result.Manager
	transport.owner.Store(result.Manager)
	answer, err := client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties.Get", "org.freedesktop.systemd1.Manager", "Version")
	if err != nil {
		return result, err
	}
	var version dbus.Variant
	if err := answer.Store(&version); err != nil || version.Signature().String() != "s" {
		return result, errors.Join(ErrObservation, err)
	}
	result.Version, ok = version.Value().(string)
	if !ok || result.Version == "" {
		return result, ErrObservation
	}
	for index := range result.Rounds {
		result.Rounds[index], err = client.round(ctx)
		if err != nil {
			return result, err
		}
	}
	var final string
	if err := client.owner(ctx, &final); err != nil || final != result.Manager {
		return result, errors.Join(ErrObservation, err)
	}
	return result, ctx.Err()
}

type startupBus struct {
	bus       *dbus.Conn
	transport *referenceTransport
	manager   string
}

func (client *startupBus) owner(ctx context.Context, owner *string) error {
	answer, err := client.request(ctx, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus.GetNameOwner", "org.freedesktop.systemd1")
	if err != nil {
		return err
	}
	return answer.Store(owner)
}

func (client *startupBus) call(ctx context.Context, object, method string, arguments ...any) (*dbus.Call, error) {
	return client.request(ctx, client.manager, object, method, arguments...)
}

func (client *startupBus) request(ctx context.Context, destination, object, method string, arguments ...any) (*dbus.Call, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	end, _ := bounded.Deadline()
	if err := client.transport.socket.SetDeadline(end); err != nil {
		return nil, err
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(bounded, func() { _ = client.transport.Close(); close(interrupted) })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	answer := client.bus.Object(destination, dbus.ObjectPath(object)).CallWithContext(bounded, method, 0, arguments...)
	if err := errors.Join(answer.Err, bounded.Err(), ctx.Err()); err != nil {
		return nil, err
	}
	return answer, nil
}

// Each complete round has six fixed read-only calls on the original bus. They
// share the original three-second bound; the next round starts only after every
// reply in this one has joined. No call changes units or retains accepting facts.
func (client *startupBus) round(ctx context.Context) (result StartupRound, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	end, _ := bounded.Deadline()
	if err := client.transport.socket.SetDeadline(end); err != nil {
		return result, err
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(bounded, func() { _ = client.transport.Close(); close(interrupted) })
	var calls [6]*dbus.Call
	var received [6]bool
	defer func() {
		if returnedErr != nil {
			cancel()
			returnedErr = errors.Join(returnedErr, client.transport.Close())
		}
		for index, call := range calls {
			if call != nil && !received[index] {
				late := <-call.Done
				returnedErr = errors.Join(returnedErr, late.Err)
			}
		}
		if !stop() {
			<-interrupted
		}
		returnedErr = errors.Join(returnedErr, bounded.Err(), ctx.Err())
		if returnedErr != nil {
			result = StartupRound{}
		}
	}()
	requests := [...]struct{ object, iface string }{
		{"/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice", "Unit"},
		{"/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice", "Service"},
		{"/org/freedesktop/systemd1/unit/ardents_2dtext_2dreader_2esocket", "Unit"},
		{"/org/freedesktop/systemd1/unit/ardents_2dtext_2dreader_2esocket", "Socket"},
		{"/org/freedesktop/systemd1/unit/ardents_2dtext_2dpublisher_2esocket", "Unit"},
		{"/org/freedesktop/systemd1/unit/ardents_2dtext_2dpublisher_2esocket", "Socket"},
	}
	for index, request := range requests {
		calls[index] = client.bus.Object(client.manager, dbus.ObjectPath(request.object)).GoWithContext(bounded,
			"org.freedesktop.DBus.Properties.GetAll", 0, make(chan *dbus.Call, 1), "org.freedesktop.systemd1."+request.iface)
	}
	values := [...]*Properties{&result.EndpointUnit, &result.EndpointService, &result.ReaderUnit, &result.ReaderSocket, &result.PublisherUnit, &result.PublisherSocket}
	for index, call := range calls {
		answer := <-call.Done
		received[index] = true
		if err := errors.Join(answer.Err, bounded.Err(), ctx.Err()); err != nil {
			return result, fmt.Errorf("fresh fixed %s properties: %w", requests[index].iface, err)
		}
		properties, err := startupProperties(answer)
		if err != nil {
			return result, err
		}
		*values[index] = properties
	}
	return result, errors.Join(bounded.Err(), ctx.Err())
}

func startupProperties(answer *dbus.Call) (Properties, error) {
	// Store's map conversion unwraps and recreates variants from reflected
	// values, losing original struct signatures (for example ExecStartEx).
	// Consume the already verified decoded dictionary without conversion.
	if len(answer.Body) != 1 {
		return nil, ErrObservation
	}
	values, ok := answer.Body[0].(map[string]dbus.Variant)
	if !ok || values == nil {
		return nil, ErrObservation
	}
	properties := make(Properties, len(values))
	encodedSize := 2 // braces of the existing bounded property dictionary
	for key, value := range values {
		body, err := managerJSONValue(value.Value(), 0)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, errors.Join(ErrObservation, err)
		}
		signature := value.Signature().String()
		encodedKey, keyErr := json.Marshal(key)
		encodedSignature, signatureErr := json.Marshal(signature)
		if keyErr != nil || signatureErr != nil {
			return nil, ErrObservation
		}
		if len(properties) != 0 {
			encodedSize++
		}
		encodedSize += len(encodedKey) + 1 + len(`{"type":`) + len(encodedSignature) + len(`,"data":`) + len(raw) + 1
		if encodedSize > 64<<10 {
			return nil, ErrObservation
		}
		properties[key] = Value{Type: signature, Data: raw}
	}
	// Preserve the existing bounded typed JSON vocabulary, including exact
	// integer values and byte arrays rather than JSON's base64 []byte form.
	// Input signatures/lengths/duplicates were already checked on the actual
	// frame. Re-encoding and decoding our own complete map adds no observation.
	return properties, nil
}

func managerJSONValue(value any, depth int) (any, error) {
	if depth > 16 {
		return nil, ErrObservation
	}
	switch typed := value.(type) {
	case dbus.Signature:
		return typed.String(), nil
	case dbus.Variant:
		body, err := managerJSONValue(typed.Value(), depth+1)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(body)
		return Value{Type: typed.Signature().String(), Data: raw}, err
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return nil, ErrObservation
	}
	switch reflected.Kind() {
	case reflect.Slice, reflect.Array:
		result := make([]any, reflected.Len())
		for index := range result {
			child, err := managerJSONValue(reflected.Index(index).Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			result[index] = child
		}
		return result, nil
	case reflect.Map:
		// busctl dictionaries preserve their ordered key/value entry shape.
		entries := make([]any, 0, reflected.Len())
		iterator := reflected.MapRange()
		for iterator.Next() {
			key, err := managerJSONValue(iterator.Key().Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			child, err := managerJSONValue(iterator.Value().Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			entries = append(entries, []any{key, child})
		}
		return entries, nil
	default:
		return value, nil
	}
}
