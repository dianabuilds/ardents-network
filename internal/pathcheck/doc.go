// Package pathcheck validates host filesystem paths and literal POSIX paths
// used in service unit declarations. Checks operate on path spellings and
// never resolve symlinks, filesystem identity, or case aliases.
package pathcheck
