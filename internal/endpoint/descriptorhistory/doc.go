// Package descriptorhistory retains one participant Context's private
// publication and Descriptor revision floors. It verifies signed proofs before
// updating a floor, remembers conflicts and expired authority until Context
// retirement, and refuses a new Target at the receiving Store's capacity.
// The Endpoint owns the Context lock and checks live authority before calling
// Accept; this package owns no network operation or durable storage.
package descriptorhistory
