// Package durablefile owns the pure durable file mechanism shared by the
// owner-only generation stores: the content-addressed generation-name grammar,
// bounded reads, exclusive creation, staged same-directory replacement,
// generation pruning, the watermark floor codec, owned-root inspection,
// directory synchronization and the exclusive root lease with its strict
// regular-file/single-link identity check.
//
// It is a mechanism leaf, not a store owner. Every helper takes caller paths,
// bytes, names and refusal prose only; it holds no lease, root, marker, schema,
// authority or lifecycle of its own. Each consuming package keeps its distinct
// marker, lock name, record schema, root and refusal text, and supplies them
// here. The shared lease identity check refuses a non-regular or multiply-linked
// lease file so a substituted inode can never become a store's exclusive fence.
//
// Responsibility clusters and their tests:
//   - generation grammar and hashing: generation_grammar.go (generation_grammar_test.go)
//   - bounded read: bounded_read.go (bounded_read_test.go)
//   - exclusive creation and staged replacement: exclusive_write.go,
//     staged_replace.go (staged_replace_test.go)
//   - generation pruning and watermark floor: generation_cleanup.go,
//     watermark.go (watermark_test.go)
//   - owned-root inspection: root_inspect.go (root_inspect_test.go)
//   - directory sync and root lease: directory_sync_unix.go,
//     directory_sync_windows.go, lease_unix.go (lease_unix_test.go)
package durablefile
