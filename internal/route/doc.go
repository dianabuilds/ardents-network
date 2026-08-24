// Package route owns native Interactive Route v1 selection and volatile
// attachment lifecycle behind Open, Attach, and Close. It creates opaque Entry
// attachments through caller-owned resource reservations, owns their cleanup,
// and provides closed v1 C-2 and one-use private-reachability setup/envelope
// codecs. It has no H3 reader or peer runtime.
package route
