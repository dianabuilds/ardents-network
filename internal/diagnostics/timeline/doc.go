// Package timeline projects bounded local Node, Source, and Endpoint runtime
// events, including Source terminal failure categories, into one operator-readable
// timeline. It accepts app JSON lines or
// journalctl JSON, drops unknown schemas, validates safe categories, and never
// retains the raw input. The command adapter owns input selection and output.
package timeline
