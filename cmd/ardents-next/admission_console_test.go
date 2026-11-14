package main

import (
	"encoding/json"
	"testing"
)

func TestAdmissionInputRejectsFieldAliasesBeforeOperations(t *testing.T) {
	for _, raw := range []string{
		`{"operation":"status","Operation":"close"}`,
		`{"operation":"status","operation":"close"}`,
		`{"operation":"begin","intent":{"Deadline":"2030-01-01T00:00:00Z","deadline":"2031-01-01T00:00:00Z"}}`,
		`{"operation":"status","payload":null}`,
		`{"unknown":"status"}`,
	} {
		var c holderCommand
		if decodeAdmissionObject([]byte(raw), &c) == nil {
			t.Fatalf("ambiguous input accepted: %s", raw)
		}
	}
	var c holderCommand
	if err := decodeAdmissionObject([]byte(`{"operation":"request","maxima":[0,4,0]}`), &c); err != nil || c.Operation != "request" || c.Maxima[1] != 4 {
		t.Fatal("positive control", err)
	}
}

func TestAdmissionFixedArraysRequireExactLength(t *testing.T) {
	for _, field := range []struct {
		name   string
		size   int
		nested bool
	}{{"maxima", 3, false}, {"digest", 32, false}, {"receivers", 32, true}} {
		for _, length := range []int{0, field.size - 1, field.size, field.size + 1} {
			values := make([]int, length)
			for i := range values {
				values[i] = i + 1
			}
			var value any = values
			if field.nested {
				value = [][]int{values}
			}
			raw, err := json.Marshal(map[string]any{"operation": "status", field.name: value})
			if err != nil {
				t.Fatal(err)
			}
			before := holderCommand{Operation: "unchanged", Digest: [32]byte{255}}
			command := before
			err = decodeAdmissionObject(raw, &command)
			if length != field.size {
				if err == nil || command.Operation != before.Operation || command.Digest != before.Digest {
					t.Fatalf("%s length %d: error=%v, operation=%q, digest=%x; want refusal before typed decoding", field.name, length, err, command.Operation, command.Digest)
				}
			} else if err != nil {
				t.Fatalf("exact %s: %v", field.name, err)
			}
		}
	}
}

func TestAdmissionVariableBytesAndOptionalArrays(t *testing.T) {
	var command holderCommand
	if err := decodeAdmissionObject([]byte(`{"operation":"import","payload":"AP8=","receivers":[]}`), &command); err != nil {
		t.Fatal(err)
	}
	if string(command.Payload) != string([]byte{0, 255}) || len(command.Receivers) != 0 || command.Maxima != [3]uint32{} {
		t.Fatal("variable bytes, empty receiver slice or omitted array changed", command)
	}
	for _, raw := range []string{`{"digest":null}`, `{"digest":"AP8="}`, `{"maxima":[1,2,3,null]}`, `{"receivers":[[1]]}`} {
		if decodeAdmissionObject([]byte(raw), &command) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
