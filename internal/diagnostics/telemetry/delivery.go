package telemetry

// DeliveryStatus describes optional diagnostic delivery, never operation success.
type DeliveryStatus string

const (
	LocalOnly   DeliveryStatus = "local-only"
	Delivered   DeliveryStatus = "completed"
	Unavailable DeliveryStatus = "unavailable"
)

// CloseWithStatus joins the exporters and classifies delivery separately from
// the operation. Pass Open's error even when it returned a nil observation.
// Nil without an opening error is the explicitly disabled local-only profile.
func (o *Observation) CloseWithStatus(openErr error) DeliveryStatus {
	if o == nil {
		if openErr != nil {
			return Unavailable
		}
		return LocalOnly
	}
	ok := o.Close()
	if openErr != nil || !ok {
		return Unavailable
	}
	return Delivered
}
