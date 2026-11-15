package systemdbus

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Independently encoded property bodies isolate decoder admission only. They
// supply no manager, installed process or successful Startup authority.
func managerDictionaryVector(keys ...string) []byte {
	body := make([]byte, 8)
	for _, key := range keys {
		for len(body)%8 != 0 {
			body = append(body, 0)
		}
		body = binary.LittleEndian.AppendUint32(body, uint32(len(key)))
		body = append(body, key...)
		body = append(body, 0, 1, 's', 0)
		for len(body)%4 != 0 {
			body = append(body, 0)
		}
		body = binary.LittleEndian.AppendUint32(body, 5)
		body = append(body, 'v', 'a', 'l', 'u', 'e', 0)
	}
	binary.LittleEndian.PutUint32(body[:4], uint32(len(body)-8))
	return body
}

func TestManagerValuesBoundNestedLengthsAndDuplicateKeys(t *testing.T) {
	valid := managerDictionaryVector("First", "Second")
	if err := ValidateValues(valid, 0, "a{sv}", binary.LittleEndian); err != nil {
		t.Fatal("independent complete typed dictionary refused", err)
	}
	for length := 0; length < len(valid); length++ {
		if err := ValidateValues(valid[:length], 0, "a{sv}", binary.LittleEndian); err == nil {
			t.Fatal("truncated nested dictionary admitted", length)
		}
	}
	oversizedArray := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(oversizedArray[:4], ^uint32(0))
	oversizedString := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(oversizedString[8:12], ^uint32(0))
	for _, invalid := range [][]byte{oversizedArray, oversizedString, managerDictionaryVector("same", "same"), append(bytes.Clone(valid), 0)} {
		if err := ValidateValues(invalid, 0, "a{sv}", binary.LittleEndian); err == nil {
			t.Fatal("unchecked nested length, duplicate or trailing bytes reached decoder")
		}
	}
	for _, signature := range []string{"h", "{sv}", "a{vs}", "a", "(", "a{}"} {
		if _, err := managerValueTypes(signature, 0); err == nil {
			t.Fatal("unsupported or malformed signature admitted", signature)
		}
	}
	boolean := []byte{2, 0, 0, 0}
	if err := ValidateValues(boolean, 0, "b", binary.LittleEndian); err == nil {
		t.Fatal("noncanonical boolean admitted")
	}
}

func TestDetachedValueBoundsRefuseInvalidInputs(t *testing.T) {
	for _, position := range []int{-1, 2} {
		if err := ValidateValues([]byte{0}, position, "u", binary.LittleEndian); err == nil {
			t.Fatalf("invalid position %d accepted", position)
		}
		if _, _, err := ReadScalar([]byte{0}, position, 1, 's', binary.LittleEndian); err == nil {
			t.Fatalf("invalid scalar position %d accepted", position)
		}
	}
	if err := ValidateValues([]byte{0}, 0, "u", nil); err == nil {
		t.Fatal("missing byte order accepted")
	}
	if _, _, err := ReadScalar([]byte{0}, 0, 1, 's', nil); err == nil {
		t.Fatal("missing scalar byte order accepted")
	}
}
