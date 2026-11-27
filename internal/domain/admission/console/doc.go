// Package console runs a local Admission conversation over bounded JSON lines
// and retains inherited descriptors through interruption and close. Endpoint
// holder and receiver owners supply the operations and retain their authority.
// Inherited descriptor duplication uses Unix; Windows refuses that operation
// before opening any Admission owner. The conversation itself is portable.
package console
