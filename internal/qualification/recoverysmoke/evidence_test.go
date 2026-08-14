package recoverysmoke

import "testing"

func TestTerminalEndpointRequiresExactlyOneResult(t *testing.T) {
	valid := []byte("diagnostic\n{\"Class\":\"deadline\"}\n")
	result, err := terminalEndpoint(valid)
	if err != nil || result.Class != "deadline" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for name, raw := range map[string][]byte{
		"missing":   []byte("diagnostic\n"),
		"duplicate": append(append([]byte(nil), valid...), []byte("{\"Class\":\"second\"}\n")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := terminalEndpoint(raw); err == nil {
				t.Fatal("invalid terminal count passed")
			}
		})
	}
}
