package keyset

// Inventory contains the public parts of the exact ordered files written.
// It carries no key handle or proof of software authorization.
type Inventory [5]Slot

// Slot pairs the published file name with its detached public key.
type Slot struct {
	File   string
	Public [32]byte
}
