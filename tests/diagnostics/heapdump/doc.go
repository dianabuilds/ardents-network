// Package heapdump is an engineering test harness for receipt-bound Go heap dumps.
// Its parser and capture verifiers run only through tests and the explicit
// heapdump-capture and heapdump-role-map Make targets; they are not runtime code.
package heapdump
