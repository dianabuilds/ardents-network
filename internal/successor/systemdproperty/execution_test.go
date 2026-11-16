package systemdproperty

import (
	"encoding/json"
	"testing"
)

func TestExecutionObservationRequiresConsistentMainProcess(t *testing.T) {
	makeValue := func(signature string, value any) Value {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return Value{Type: signature, Data: body}
	}
	service := Properties{
		"ExecStartEx":            makeValue("a(sasasttttuii)", []any{[]any{"/program", []string{"/program"}, []string{}, 10, 20, 0, 0, 7317, 0, 0}}),
		"ExecMainStartTimestamp": makeValue("t", 30), "ExecMainStartTimestampMonotonic": makeValue("t", 40),
		"ExecMainExitTimestamp": makeValue("t", 0), "ExecMainExitTimestampMonotonic": makeValue("t", 0),
		"ExecMainPID": makeValue("u", 7317), "ExecMainCode": makeValue("i", 0), "ExecMainStatus": makeValue("i", 0),
	}
	got, ok := ReadExecution(service)
	if !ok || got != (Execution{30, 40, 0, 0, 7317, 0, 0}) {
		t.Fatal("original running observation refused", got, ok)
	}
	for _, change := range []struct {
		name  string
		value Value
	}{
		{"ExecMainPID", makeValue("u", 7318)},
		{"ExecMainStartTimestamp", makeValue("t", 9)},
		{"ExecMainStartTimestampMonotonic", makeValue("t", 0)},
		{"ExecMainExitTimestamp", makeValue("t", 1)},
		{"ExecMainStatus", makeValue("u", 0)},
		{"ExecStartEx", makeValue("a(sasasttttuii)", []any{})},
	} {
		original := service[change.name]
		service[change.name] = change.value
		if _, ok := ReadExecution(service); ok {
			t.Fatal("inconsistent observation accepted", change.name)
		}
		service[change.name] = original
	}
	delete(service, "ExecMainCode")
	if _, ok := ReadExecution(service); ok {
		t.Fatal("missing execution field accepted")
	}
}
