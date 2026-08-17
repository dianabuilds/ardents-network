package modulecache

// Options identify the repository and a new external archive path.
type Options struct {
	Workspace string
	Output    string
}

// Receipt binds the completed canonical archive.
type Receipt struct {
	SHA256 string
	Bytes  int64
}
