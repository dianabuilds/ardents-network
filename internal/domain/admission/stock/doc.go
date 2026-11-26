// Package stock owns volatile holder secrets, pending issuance reservations,
// exact retries and finalized tokens. Explicit attempts contain only public
// request access; the application executes and joins transport independently.
// Stock consumes an authority observer. Open owns its presentation journal;
// New borrows a caller-owned journal for composition with an existing lease.
package stock
