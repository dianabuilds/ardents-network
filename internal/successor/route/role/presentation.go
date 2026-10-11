package role

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Presentation returns genuinely durably presented Stock bytes. The supplying
// Admission owner retains spending policy; a transport refusal cannot refund it.
type Presentation func(context.Context, ardp.Hello) ([]byte, error)

// AdmissionWireBytes charges HELLO, ADMIT and ACCEPT before admitted work.
const AdmissionWireBytes = 3*ardp.HeaderSize + 209 + 355 + 5

// Binding derives the local channel binding from the exact HELLO and the actual
// negotiated role exporter. It conveys no Admission right or path-wide identity.
func Binding(conn net.Conn, h ardp.Hello) ([32]byte, error) {
	body, err := ardp.EncodeHello(h)
	if err != nil {
		return [32]byte{}, err
	}
	export, err := transport.ClosedRoleTLSExporter(conn)
	if err != nil {
		return [32]byte{}, err
	}
	digest := sha256.Sum256(body)
	raw, err := export("EXPORTER-ardents-channel-v3", digest[:], 32)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(raw)
	if len(raw) != 32 {
		return [32]byte{}, errors.New("route exporter unavailable")
	}
	var binding [32]byte
	copy(binding[:], raw)
	return binding, nil
}

// Present performs the holder's HELLO/presentation/ADMIT/ACCEPT stages. Original
// caller and duty checks surround I/O; the caller owns physical interruption and
// joined cleanup. Receiving Grant transfer belongs to the receiving owner.
func Present(ctx, caller context.Context, conn net.Conn, a Authority, h ardp.Hello, presentation Presentation) error {
	if ctx == nil || caller == nil || conn == nil || presentation == nil {
		return errors.New("route presentation composition unavailable")
	}
	check := func() error {
		if err := errors.Join(ctx.Err(), caller.Err()); err != nil {
			return err
		}
		_, err := a.Hello(h, false)
		// Observation may perform I/O. Recheck original cancellation after it.
		return errors.Join(err, ctx.Err(), caller.Err())
	}
	if _, err := Binding(conn, h); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := channel.SendHello(conn, h); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	raw, err := presentation(ctx, h)
	if err != nil {
		return err
	}
	defer clear(raw)
	if len(raw) != 354 {
		return errors.New("route token length invalid")
	}
	class, err := AdmissionClass(h.Purpose)
	if err != nil {
		return err
	}
	body := append([]byte{byte(class)}, raw...)
	defer clear(body)
	if err := check(); err != nil {
		return err
	}
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: body}); err != nil {
		return err
	}
	if err := channel.Accepted(conn); err != nil {
		return err
	}
	return check()
}

// AdmissionClass binds the implemented role purposes to existing Admission
// classes. It creates no class and authorizes no receiving verification.
func AdmissionClass(purpose ardp.Purpose) (admission.Class, error) {
	switch purpose {
	case ardp.PurposeIssuer, ardp.PurposeReachability, ardp.PurposeSubmission:
		return admission.ControlClass, nil
	case ardp.PurposeForwarding, ardp.PurposeDataJoin:
		return admission.ForwardClass, nil
	case ardp.PurposeIntroduction:
		return admission.RegistrationClass, nil
	default:
		return 0, errors.New("route terminal purpose unavailable")
	}
}
