// Package pairing owns one exact receiving JOIN duty's Rendezvous matching:
// two complete RESULT writes before any data, relay accounting/credit and the
// joined physical retirement of both original sides. Each Pairing owns both
// physical sides until their readers and writers have returned. Capacity
// reserves one side's bounded frame memory from the receiving principal's
// shared budget before irreversible admission spend and releases it exactly
// once after Serve (or refused admission physical cleanup) has joined.
//
// Receiver supplies the admitted channel, original allowance and currentness
// check and retains Grant/Hosting return until Serve completes; pairing neither
// verifies nor releases an Admission Grant. It owns no durable root, generation,
// selection, listener or native mechanism. interruptIO, when supplied, belongs
// to the exact admitted lower lane and performs no new deadline I/O after
// framing already stopped. Setup, relay and terminal cleanup use portable
// ordered I/O, context, sync and the original absolute bounds, with Windows and
// Linux tests. Holder acquisitions, Context and Joined stream remain in the
// parent join package.
package pairing
