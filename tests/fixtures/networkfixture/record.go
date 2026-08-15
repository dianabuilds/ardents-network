package networkfixture

import (
	"bytes"
	"crypto/ed25519"
	"errors"
)

// BuildRecord returns one canonical signed Node Record.
func BuildRecord(spec RecordSpec) (Record, error) {
	if spec.Generation == 0 || spec.ValidFrom.IsZero() || !spec.ValidUntil.After(spec.ValidFrom) ||
		spec.Family == "" || len(spec.Family) > 32 || spec.Endpoint == "" || len(spec.Endpoint) > 96 ||
		len(spec.PrivateKey) != ed25519.PrivateKeySize {
		return Record{}, errors.New("record fixture specification is invalid")
	}
	public := spec.PrivateKey.Public().(ed25519.PublicKey)
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARNR")
	buffer.WriteByte(1)
	buffer.Write(spec.NetworkID[:])
	buffer.Write(spec.NodeID[:])
	u64(buffer, spec.Generation)
	i64(buffer, spec.ValidFrom.Unix())
	i64(buffer, spec.ValidUntil.Unix())
	text(buffer, spec.Family)
	buffer.WriteByte(spec.Capability)
	text(buffer, spec.Endpoint)
	u16(buffer, spec.Capacity)
	buffer.Write(public)
	buffer.Write(ed25519.Sign(spec.PrivateKey, buffer.Bytes()))
	return Record{Raw: buffer.Bytes(), NodeID: spec.NodeID, Family: spec.Family, Capacity: spec.Capacity}, nil
}
