package state

import "github.com/dianabuilds/ardents-network/internal/network/framing"

type decoder struct {
	*framing.Reader
	length int
}

func newDecoder(raw []byte) decoder { return decoder{Reader: framing.New(raw), length: len(raw)} }

func (d *decoder) bytes(length int) ([]byte, error) { return d.Bytes(length) }
func (d *decoder) byte() (byte, error) {
	value, err := d.Bytes(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}
func (d *decoder) uint16() (uint16, error) { return d.Uint16() }
func (d *decoder) uint32() (uint32, error) { return d.Uint32() }
func (d *decoder) uint64() (uint64, error) { return d.Uint64() }
func (d *decoder) done() bool              { return d.Consumed() == d.length }
