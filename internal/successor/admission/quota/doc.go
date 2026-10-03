// Package quota owns the issuer duty's durable permission debits, hourly
// limits and exact retries. Only its committed transaction can create a
// DebitConfirmation. Keys, signatures and physical capacity are not owned here.
package quota
