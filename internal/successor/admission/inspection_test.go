package admission

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"math"
	"testing"
	"time"
)

// Construct wire bytes from the documented offsets, without a production encoder.
func fixture() ([]byte, Facts, ed25519.PrivateKey) {
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	f := Facts{Duty: 7, Class: 2, Count: 11, DutyNotBefore: time.Unix(3600, 0), DutyNotAfter: time.Unix(7200, 0), Now: time.Unix(3600, 0)}
	f.Network[0] = 1
	f.Issuer[0] = 2
	f.Holder[0] = 3
	copy(f.Authority[:], key.Public().(ed25519.PublicKey))
	raw := make([]byte, 228)
	copy(raw[:32], f.Network[:])
	copy(raw[32:64], f.Issuer[:])
	binary.BigEndian.PutUint64(raw[64:72], 7)
	raw[72] = 4
	copy(raw[104:136], f.Holder[:])
	binary.BigEndian.PutUint64(raw[136:144], 3600)
	binary.BigEndian.PutUint64(raw[144:152], 7200)
	binary.BigEndian.PutUint32(raw[156:160], 11)
	copy(raw[164:], ed25519.Sign(key, append([]byte("ardents-issuance-permission-v1\x00"), raw[:164]...)))
	return raw, f, key
}

func TestInspection(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func([]byte, *Facts)
		want   Outcome
	}{
		{"valid", func([]byte, *Facts) {}, Accepted},
		{"expiry exclusive", func(_ []byte, f *Facts) { f.Now = f.DutyNotAfter }, Validity},
		{"before start", func(_ []byte, f *Facts) { f.Now = f.Now.Add(-time.Nanosecond) }, Validity},
		{"last valid instant", func(_ []byte, f *Facts) { f.Now = f.DutyNotAfter.Add(-time.Nanosecond) }, Accepted},
		{"wrong holder", func(_ []byte, f *Facts) { f.Holder[0]++ }, Binding},
		{"wrong network", func(_ []byte, f *Facts) { f.Network[0]++ }, Binding},
		{"wrong issuer", func(_ []byte, f *Facts) { f.Issuer[0]++ }, Binding},
		{"wrong duty", func(_ []byte, f *Facts) { f.Duty++ }, Binding},
		{"over quota", func(_ []byte, f *Facts) { f.Count++ }, Limit},
		{"empty class", func(_ []byte, f *Facts) { f.Class = 1 }, Limit},
		{"invalid class", func(_ []byte, f *Facts) { f.Class = 4 }, InvalidInput},
		{"wrong authority", func(_ []byte, f *Facts) { f.Authority[0]++ }, Signature},
		{"tamper", func(b []byte, _ *Facts) { b[164] ^= 1 }, Signature},
		{"integer overflow", func(b []byte, _ *Facts) {
			// Keep unsigned hour alignment and a wrapped end, so another grammar
			// check cannot conceal a missing overflow guard.
			start := uint64(math.MaxUint64) / 3600 * 3600
			binary.BigEndian.PutUint64(b[136:144], start)
			binary.BigEndian.PutUint64(b[144:152], start+3600)
		}, Malformed},
		{"duty interval", func(_ []byte, f *Facts) { f.DutyNotAfter = f.DutyNotAfter.Add(-time.Second) }, Validity},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, f, _ := fixture()
			test.change(raw, &f)
			if got := Inspect(context.Background(), raw, f); got != test.want {
				t.Fatalf("got %s want %s", got, test.want)
			}
		})
	}
	raw, f, _ := fixture()
	for _, size := range []int{0, 163, 227, 229} {
		b := make([]byte, size)
		copy(b, raw)
		if got := Inspect(context.Background(), b, f); got != Malformed {
			t.Fatalf("size %d: %s", size, got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Inspect(ctx, raw, f) != Canceled {
		t.Fatal("cancellation ignored")
	}
}

func TestSignedPermissionLimits(t *testing.T) {
	for _, maximum := range []uint32{0, 65536, 65537} {
		for class := uint8(1); class <= 3; class++ {
			raw, f, key := fixture()
			clear(raw[152:164])
			binary.BigEndian.PutUint32(raw[152+int(class-1)*4:156+int(class-1)*4], maximum)
			copy(raw[164:], ed25519.Sign(key, append([]byte("ardents-issuance-permission-v1\x00"), raw[:164]...)))
			f.Class, f.Count = class, 65536
			want := Malformed
			if maximum == 65536 {
				want = Accepted
			}
			if got := Inspect(context.Background(), raw, f); got != want {
				t.Fatalf("class %d maximum %d: %s", class, maximum, got)
			}
		}
	}
}

func TestEpochHourIsValidOffline(t *testing.T) {
	raw, f, key := fixture()
	binary.BigEndian.PutUint64(raw[136:144], 0)
	binary.BigEndian.PutUint64(raw[144:152], 3600)
	copy(raw[164:], ed25519.Sign(key, append([]byte("ardents-issuance-permission-v1\x00"), raw[:164]...)))
	f.DutyNotBefore, f.Now = time.Unix(0, 0), time.Unix(0, 0)
	f.DutyNotAfter = time.Unix(3600, 0)
	if got := Inspect(context.Background(), raw, f); got != Accepted {
		t.Fatalf("canonical Unix epoch hour refused: %s", got)
	}
}

func TestSignatureCoversAllFields(t *testing.T) {
	raw, f, _ := fixture()
	for _, offset := range []int{0, 32, 64, 72, 104, 136, 144, 152, 156, 160} {
		b := append([]byte(nil), raw...)
		b[offset] ^= 1
		if Inspect(context.Background(), b, f) == Accepted {
			t.Fatalf("accepted mutation at %d", offset)
		}
	}
}

func TestWellFormedTamperingFailsSignature(t *testing.T) {
	raw, f, _ := fixture()
	// These mutations remain structurally valid, so grammar refusal cannot hide
	// a missing signed field. Change the interval as one aligned hour as well.
	for _, offset := range []int{31, 63, 71, 103, 135, 155, 159, 163} {
		b := append([]byte(nil), raw...)
		b[offset] ^= 1
		if got := Inspect(context.Background(), b, f); got != Signature {
			t.Fatalf("field ending at %d: got %s", offset, got)
		}
	}
	b := append([]byte(nil), raw...)
	binary.BigEndian.PutUint64(b[136:144], 7200)
	binary.BigEndian.PutUint64(b[144:152], 10800)
	if got := Inspect(context.Background(), b, f); got != Signature {
		t.Fatalf("aligned interval tampering: %s", got)
	}
}
