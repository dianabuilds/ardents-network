// Package admission verifies closed permissions and signed issuance batches
// against explicit offline evidence, and owns durable nonrefundable quota
// debits in isolated fresh roots. It creates no live State/time authority,
// network permission, signing key or token.
package admission
