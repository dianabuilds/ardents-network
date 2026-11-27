// Package socketfile binds a local Application listener and retains its
// original directory entry. Administration and Connection own accepted
// clients and join them before retiring that entry. The caller supplies a
// private local directory; identity checks do not serialize other writers
// with access to that directory.
package socketfile
