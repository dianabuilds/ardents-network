package worker

// Activation is the mechanism result of one installed launch attempt. It is
// not a launch receipt: readiness exchange, job authority and Grant binding
// belong to the composition that called Activate. Unsupported native adapters
// return no Activation, including no queued obligation.
type Activation struct {
	// Queued retains a possibly accepted activation even after a failed dial.
	Queued bool
	// Artifact pins the root-installed artifact bytes.
	Artifact *Artifact
	// Instance is populated only after actual manager-owned observation.
	Instance Instance
	// Attachment retains the original stream. Until successful observation it
	// has no worker identity and permits only the caller's physical cleanup.
	Attachment *Attachment
}
