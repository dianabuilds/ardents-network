package systemdproperty

import (
	"encoding/json"
	"testing"
)

func TestReadInvocationRequiresExactOriginalTypedValues(t *testing.T) {
	unit := Properties{"InvocationID": {Type: "ay", Data: json.RawMessage(`[9,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}}
	service := Properties{"MainPID": {Type: "u", Data: json.RawMessage(`7317`)}}
	pid, id, ok := ReadInvocation(unit, service)
	if !ok || pid != 7317 || id != [16]byte{9} {
		t.Fatal("valid observation refused", pid, id, ok)
	}
	for _, raw := range []string{`[]`, `[9]`, `null`, `"CQAAAAAAAAAAAAAAAAAAAA=="`, `[9,null,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`, `[256,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`, `[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`} {
		unit["InvocationID"] = Value{Type: "ay", Data: json.RawMessage(raw)}
		if _, _, ok := ReadInvocation(unit, service); ok {
			t.Fatal("invalid invocation accepted", raw)
		}
	}
	unit["InvocationID"] = Value{Type: "ay", Data: json.RawMessage(`[9,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}
	for _, value := range []Value{{Type: "u", Data: json.RawMessage(`0`)}, {Type: "t", Data: json.RawMessage(`7317`)}} {
		service["MainPID"] = value
		if _, _, ok := ReadInvocation(unit, service); ok {
			t.Fatal("invalid main PID accepted", value)
		}
	}
}
