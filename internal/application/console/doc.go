// Package console owns the local presentation descriptors used by Application
// commands. The Linux implementation reopens the original descriptors without
// changing the invoking shell's flags and requires interruptible I/O. Other
// platforms refuse the unavailable host boundary.
package console
