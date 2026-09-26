// Package duty owns bounded Endpoint-local Role Domain duty and exposure truth.
// Its durable version-1 root still decodes the retained receiving one-use
// Transit Grant spend ledger records; the spend operation that wrote them was
// retired with the Route v2 execution closure (ADR-0093), and the persisted
// schema keeps its own separate data disposition.
package duty
