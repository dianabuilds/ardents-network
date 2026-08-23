package route

import (
	"errors"
	"io"
)

// ConfirmNodeLegBinding sends local's exact native Node-to-Node LegBinding
// then requires the reciprocal peer record. Call it only after the adjacent
// TLS 1.3 handshake selected the native Profile.
func ConfirmNodeLegBinding(connection io.ReadWriter, local LegBinding) error {
	if connection == nil {
		return errors.New("native Route leg connection is unavailable")
	}
	raw, err := EncodeLegBinding(local)
	if err != nil {
		return err
	}
	if err := writeAll(connection, raw); err != nil {
		return err
	}
	peer, err := readNodeLegBinding(connection)
	if err != nil {
		return err
	}
	return local.VerifyReciprocal(peer)
}

// AcceptNodeLegBinding receives a peer's native Node-to-Node LegBinding,
// verifies it is reciprocal to local, then returns local's one response. It
// never accepts legacy H3 framing or a Node-selected Profile/version.
func AcceptNodeLegBinding(connection io.ReadWriter, local LegBinding) error {
	if connection == nil {
		return errors.New("native Route leg connection is unavailable")
	}
	peer, err := readNodeLegBinding(connection)
	if err != nil {
		return err
	}
	if err := local.VerifyReciprocal(peer); err != nil {
		return err
	}
	raw, err := EncodeLegBinding(local)
	if err != nil {
		return err
	}
	return writeAll(connection, raw)
}

func readNodeLegBinding(reader io.Reader) (LegBinding, error) {
	header := make([]byte, len(routeWireMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return LegBinding{}, err
	}
	if string(header[:len(routeWireMagic)]) != routeWireMagic {
		return LegBinding{}, errors.New("native Route leg magic is invalid")
	}
	length := int(header[len(routeWireMagic)])<<8 | int(header[len(routeWireMagic)+1])
	if length == 0 || length > maximumWireBody {
		return LegBinding{}, errors.New("native Route leg length is invalid")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return LegBinding{}, err
	}
	return DecodeLegBinding(append(header, body...))
}
