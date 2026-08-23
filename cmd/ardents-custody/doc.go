// Command ardents-custody is the separate custody-process adapter. It exposes
// canonical public envelope metadata and can verify one encrypted record after
// reading a password only from an interactive no-echo terminal. It never
// accepts a password or decrypted Authority material through arguments,
// configuration, environment, or standard input shared with application data.
package main
