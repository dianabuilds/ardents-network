// Package applicationv1 contains generated Application protocol types and wire limits.
// It does not own public SDK behavior or server-side Application policy.
package applicationv1

const (
	// MaxUnaryPayloadBytes is the largest content body accepted by the unary API.
	MaxUnaryPayloadBytes = 4 << 20
	// MaxUnaryMessageBytes leaves bounded room for protobuf fields around the body.
	MaxUnaryMessageBytes = MaxUnaryPayloadBytes + 64<<10
)
