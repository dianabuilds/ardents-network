package systemd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// Reference-frame vectors use the production decoder's scalar-only vocabulary.
func verifyReferenceFrame(frame []byte, owner string) error {
	return verifyManagerFrame(frame, owner, false)
}

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
	if err := verifyManagerValues(valid, 0, "a{sv}", binary.LittleEndian); err != nil {
		t.Fatal("independent complete typed dictionary refused", err)
	}
	for length := 0; length < len(valid); length++ {
		if err := verifyManagerValues(valid[:length], 0, "a{sv}", binary.LittleEndian); err == nil {
			t.Fatal("truncated nested dictionary admitted", length)
		}
	}
	oversizedArray := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(oversizedArray[:4], ^uint32(0))
	oversizedString := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(oversizedString[8:12], ^uint32(0))
	for _, invalid := range [][]byte{oversizedArray, oversizedString, managerDictionaryVector("same", "same"), append(bytes.Clone(valid), 0)} {
		if err := verifyManagerValues(invalid, 0, "a{sv}", binary.LittleEndian); err == nil {
			t.Fatal("unchecked nested length, duplicate or trailing bytes reached decoder")
		}
	}
	for _, signature := range []string{"h", "{sv}", "a{vs}", "a", "(", "a{}"} {
		if _, err := managerValueTypes(signature, 0); err == nil {
			t.Fatal("unsupported or malformed signature admitted", signature)
		}
	}
	boolean := []byte{2, 0, 0, 0}
	if err := verifyManagerValues(boolean, 0, "b", binary.LittleEndian); err == nil {
		t.Fatal("noncanonical boolean admitted")
	}
}

func TestManagerJSONPreservesBytesAndExactIntegerValues(t *testing.T) {
	for _, test := range []struct {
		input    any
		expected string
	}{
		{[]byte{0, 255}, "[0,255]"},
		{[]byte{}, "[]"},
		{[]any{uint64(1<<63 + 7), "original"}, "[9223372036854775815,\"original\"]"},
	} {
		value, err := managerJSONValue(test.input, 0)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(value)
		if err != nil || string(raw) != test.expected {
			t.Fatal("typed native facts changed representation", string(raw), err)
		}
	}
}

func TestReadOnlyManagerReplyCannotBroadenReferenceAdmission(t *testing.T) {
	body := managerDictionaryVector("Original")
	frame := make([]byte, 16)
	frame[0], frame[1], frame[3] = 'l', 2, 1
	frame = append(frame, 5, 1, 'u', 0, 1, 0, 0, 0)
	frame = append(frame, 7, 1, 's', 0, 4, 0, 0, 0, ':', '1', '.', '2', 0)
	for len(frame)%8 != 0 {
		frame = append(frame, 0)
	}
	frame = append(frame, 8, 1, 'g', 0, 5, 'a', '{', 's', 'v', '}', 0)
	binary.LittleEndian.PutUint32(frame[12:16], uint32(len(frame)-16))
	for len(frame)%8 != 0 {
		frame = append(frame, 0)
	}
	frame = append(frame, body...)
	binary.LittleEndian.PutUint32(frame[4:8], uint32(len(body)))
	if err := verifyManagerFrame(frame, ":1.2", true); err != nil {
		t.Fatal("independent original property reply refused", err)
	}
	if err := verifyReferenceFrame(frame, ":1.2"); err == nil {
		t.Fatal("property vocabulary expanded the scalar reference client")
	}
	if err := verifyManagerFrame(frame, ":1.3", true); err == nil {
		t.Fatal("foreign manager supplied original observations")
	}
	oversized := bytes.Clone(frame)
	binary.LittleEndian.PutUint32(oversized[len(frame)-len(body):], ^uint32(0))
	if err := verifyManagerFrame(oversized, ":1.2", true); err == nil {
		t.Fatal("original property frame admitted unchecked nested allocation")
	}
}
