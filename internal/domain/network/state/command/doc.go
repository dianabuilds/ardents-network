// Package command adapts local Network State intake and retained inspection.
// Each offline operation opens the original State and joins it before its
// receipt. State owns signed admission, floors, locks and current observations.
// The command reads declarations and returns detached reports to ardents-next.
package command
