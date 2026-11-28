// Package localread runs the local document reader. It reads one Target Link,
// dials the local Endpoint, and writes escaped text after the document stream
// completes. It owns the input and output descriptors until closure joins.
package localread
