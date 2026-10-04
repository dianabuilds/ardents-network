package source

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

func TestBundleFrozenWireAndOwnedDecode(t *testing.T) {
	bundle := Bundle{Epoch: []byte{0xaa}, Inputs: [][]byte{{0xbb, 0xcc}}, Materials: [][]byte{{0xdd}}}
	raw, err := EncodeBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	const frozen = "415244483342310000000001aa000100000002bbcc000100000001dd"
	if hex.EncodeToString(raw) != frozen {
		t.Fatalf("bundle wire changed: %x", raw)
	}
	decoded, err := DecodeBundle(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw[12], raw[19], raw[27] = 0, 0, 0
	if !bytes.Equal(decoded.Epoch, bundle.Epoch) || !bytes.Equal(decoded.Inputs[0], bundle.Inputs[0]) || !bytes.Equal(decoded.Materials[0], bundle.Materials[0]) {
		t.Fatalf("decoded bundle aliases wire input: %+v", decoded)
	}
}

func TestDecodeBundleRejectsMalformedFraming(t *testing.T) {
	valid, err := hex.DecodeString("415244483342310000000001aa000100000002bbcc000100000001dd")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":              nil,
		"oversize":           make([]byte, maximumPayloadBytes+1),
		"truncated":          valid[:len(valid)-1],
		"trailing":           append(bytes.Clone(valid), 0),
		"wrong magic":        append([]byte{0}, valid[1:]...),
		"zero epoch length":  append(bytes.Clone(valid[:8]), append([]byte{0, 0, 0, 0}, valid[12:]...)...),
		"too many inputs":    append(bytes.Clone(valid[:13]), append([]byte{0, 65}, valid[15:]...)...),
		"too many materials": append(bytes.Clone(valid[:21]), append([]byte{0, 65}, valid[23:]...)...),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBundle(raw); !errors.Is(err, ErrFraming) {
				t.Fatalf("malformed bundle cause=%v; want framing", err)
			}
		})
	}
}

func TestEncodeBundleEnforcesPrivateResponseBounds(t *testing.T) {
	base := Bundle{Epoch: []byte{1}, Materials: [][]byte{{2}}}
	cases := map[string]Bundle{
		"missing epoch":             {Materials: base.Materials},
		"missing selected material": {Epoch: base.Epoch},
		"multiple materials":        {Epoch: base.Epoch, Materials: [][]byte{{1}, {2}}},
		"empty input":               {Epoch: base.Epoch, Inputs: [][]byte{{}}, Materials: base.Materials},
		"large input":               {Epoch: base.Epoch, Inputs: [][]byte{make([]byte, epoch2.MaxRecordBytes+1)}, Materials: base.Materials},
		"large material":            {Epoch: base.Epoch, Materials: [][]byte{make([]byte, epoch2.MaxMaterializationBytes+1)}},
		"large payload":             {Epoch: bytes.Repeat([]byte{1}, maximumPayloadBytes), Materials: base.Materials},
	}
	for name, bundle := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := EncodeBundle(bundle); err == nil {
				t.Fatal("encoded out-of-bounds bundle")
			}
		})
	}
}

func TestDecodeBundleLeavesMaterialSelectionToState(t *testing.T) {
	raw, err := hex.DecodeString("415244483342310000000001aa000100000002bbcc0000")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := DecodeBundle(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Materials) != 0 {
		t.Fatalf("material count = %d", len(bundle.Materials))
	}
	if _, err := EncodeBundle(bundle); err == nil {
		t.Fatalf("Source encoder accepted no selected material: %v", err)
	}
}
