package updatetransaction

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestSchemaCurrentRoundTripAndRejectsDescriptorForgery(t *testing.T) {
	owner := sha256.Sum256([]byte("schema-owner"))
	content := sha256.Sum256([]byte("schema-content"))
	selection := SchemaSelection{Owner: owner, Generation: 7, Content: content, Bytes: 4096, Entries: 3}
	selection.Identity = schemaSelectionIdentity(selection)
	predecessor := sha256.Sum256([]byte("prior-schema-record"))
	want := schemaCurrent{Transaction: 9, Selection: selection, Predecessor: predecessor}

	raw, err := encodeSchemaCurrent(want)
	if err != nil || len(raw) != recordHeaderBytes+schemaRecordBodyBytes {
		t.Fatalf("encode schema current len=%d err=%v", len(raw), err)
	}
	got, err := decodeSchemaCurrent(raw)
	if err != nil || got != want {
		t.Fatalf("decode schema current=%+v err=%v", got, err)
	}
	for _, offset := range []int{8, 48, 80, 112, 120} {
		forged := append([]byte(nil), raw...)
		forged[recordHeaderBytes+offset] ^= 0xff
		if _, err := decodeSchemaCurrent(forged); err == nil {
			t.Fatalf("decode accepted mutation at body offset %d", offset)
		}
	}
	if !bytes.Equal(raw[:8], []byte("ARDUPD01")) {
		t.Fatal("schema record did not use owned envelope")
	}
}
