package enrollment

import "os"

// Portable Windows verification retains the predecessor's regular-file,
// identity and byte policy; it does not attest Unix ownership or installed ACLs.
func verifyOwnedFile(os.FileInfo) error { return nil }

func openBundleFile(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
