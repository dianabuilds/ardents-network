// Package node owns the bounded black-box Node qualification campaign. It
// classifies candidate violations separately from missing or corrupt harness
// evidence. A passing campaign covers only the gates named by its sealed
// manifest and receipts; it is not by itself a Horizon stage verdict.
package node
