# Separate Application and Operator interfaces

Ardents exposes Applications through a new capability-scoped, versioned Application Interface instead of publishing or wrapping the existing Operator Interface. This keeps administrative lifecycle, configuration, diagnostics, and workload authority out of SDK credentials while allowing the wire contract and language SDKs to evolve around application use cases; the cost is a separate protocol, authorization catalogue, listener configuration, and adapter layer.
