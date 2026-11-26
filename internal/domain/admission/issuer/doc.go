// Package issuer composes offline and retained current-authority token issuance,
// and separately pinned issuer-profile provisioning.
// Admission owns quota decisions and Issuance owns keys and durable responses.
// An original issuing lifetime joins its operations before results, keys and
// quota retire; it owns no Network authentication, process protection or Route.
package issuer
