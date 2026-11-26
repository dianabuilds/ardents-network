package unit

import (
	"encoding/json"
	"testing"
)

func TestPlatformCompletionRequiresExactSuccessfulStoppedExecution(t *testing.T) {
	for _, fault := range []string{"none", "queued", "live", "foreign-exec", "lost-history", "reversed-clocks", "failed-exit"} {
		t.Run(fault, func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			program := request.InstallationRoot + "/generations/" + digest + "/ardents-linux-amd64"
			execution := []any{program, []string{program, "endpoint", "start-installed", request.InstallationRoot},
				[]string{"no-env-expand"}, 10, 20, 30, 40, 123, 1, 0}
			unit["Job"] = managerValue{Type: "(uo)", Data: json.RawMessage(`[0,"/"]`)}
			service["Result"] = managerValue{Type: "s", Data: json.RawMessage(`"success"`)}
			switch fault {
			case "queued":
				unit["Job"] = managerValue{Type: "(uo)", Data: json.RawMessage(`[1,"/org/freedesktop/systemd1/job/1"]`)}
			case "live":
				service["MainPID"] = managerValue{Type: "u", Data: json.RawMessage(`123`)}
			case "foreign-exec":
				execution[0] = "/other/ardents"
			case "lost-history":
				execution[7] = 0
			case "reversed-clocks":
				execution[6] = 19
			case "failed-exit":
				execution[9] = 1
			}
			body, err := json.Marshal([][]any{execution})
			if err != nil {
				t.Fatal(err)
			}
			service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: body}
			err = VerifyPlatformCompleted("255.4", unit, service, request, digest, platformTestCheckProgram, platformTestManifest, platformTestPin())
			if (err == nil) != (fault == "none") {
				t.Fatalf("platform completion for %s: %v", fault, err)
			}
		})
	}
}
