// Package counters observes the original host boot, network interface
// identities and byte totals. Linux reads procfs and sysfs. macOS reads
// the 64-bit interface counters from IFMIB. Other platforms return typed
// unavailability. Hosting owns durable continuity and budget decisions.
package counters
