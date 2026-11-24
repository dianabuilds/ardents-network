package node_test

import (
	"encoding/json"
	"testing"

	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
)

func TestAdmissionConsoleInputGrammar(t *testing.T) {
	binary, _, config := admissionConsoleFixture(t)
	send, closeConsole := nodefixture.OpenAdmissionConsole(t, binary, "holder", config)
	defer closeConsole()
	for _, raw := range []string{
		`{"operation":"status","Operation":"close"}`,
		`{"operation":"status","operation":"close"}`,
		`{"operation":"begin","intent":{"Deadline":"2030-01-01T00:00:00Z","deadline":"2031-01-01T00:00:00Z"}}`,
		`{"operation":"status","payload":null}`,
		`{"unknown":"status"}`,
		`{"operation":"status","digest":null}`,
		`{"operation":"status","digest":"AP8="}`,
		`{"operation":"status","maxima":[1,2,3,null]}`,
		`{"operation":"status","receivers":[[1]]}`,
	} {
		if reply := send(json.RawMessage(raw)); reply.Outcome != "refused" {
			t.Fatalf("ambiguous input %s: %+v", raw, reply)
		}
	}
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
			want := "refused"
			if length == field.size {
				want = "completed"
			}
			if reply := send(map[string]any{"operation": "status", field.name: value}); reply.Outcome != want {
				t.Fatalf("%s length %d: got %q, want %q", field.name, length, reply.Outcome, want)
			}
		}
	}
	if reply := send(json.RawMessage(`{"operation":"status","payload":"AP8=","receivers":[]}`)); reply.Outcome != "completed" {
		t.Fatal("variable bytes and empty receiver list refused", reply)
	}
}
