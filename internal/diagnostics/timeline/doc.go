// Package timeline projects bounded local Node, Source, and Endpoint runtime
// events, including Source terminal failure categories, into one operator-readable
// timeline. It accepts app JSON lines or
// journalctl JSON, drops unknown schemas, rejects corrupt input and unsafe
// categories without echoing raw bytes, and never retains the raw input. The
// command adapter owns input selection and output.
package timeline
