package systemdproperty

import "encoding/json"

// ReadInvocation decodes a detached main PID and systemd invocation ID.
// A matching native process and original lifecycle still require their own checks.
func ReadInvocation(unit, service Properties) (uint32, [16]byte, bool) {
	var pid uint32
	var id [16]byte
	main, mainPresent := service["MainPID"]
	invocation, idPresent := unit["InvocationID"]
	var parts []json.RawMessage
	if !mainPresent || !idPresent || main.Type != "u" || invocation.Type != "ay" || Decode(main.Data, &pid) != nil || pid == 0 || Decode(invocation.Data, &parts) != nil || len(parts) != len(id) {
		return 0, [16]byte{}, false
	}
	for index, part := range parts {
		if Decode(part, &id[index]) != nil {
			return 0, [16]byte{}, false
		}
	}
	if id == [16]byte{} {
		return 0, [16]byte{}, false
	}
	return pid, id, true
}
