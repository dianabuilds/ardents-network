//go:build ignore

package main

import "fmt"

// personaPrompt is the complete decision surface exposed to an LLM. The
// coordinator owns the container, command, plan, state roots, sequence, and
// evidence destination; unknown JSON fields are rejected before execution.
func personaPrompt(persona string) string {
	goal := map[string]string{
		"honest_user":    "Request refresh while checking that authenticated State remains available.",
		"battery_saver":  "Request one refresh, then prefer noop when the accepted generation has not changed.",
		"probe_consumer": "Request one refresh to observe whether the intentionally invalid second Source is rejected, then choose noop for the rest of this three-minute observation window because that rejection intentionally activates durable backoff.",
	}[persona]
	return fmt.Sprintf("Persona: %s\nGoal: %s\nReturn exactly one JSON object using schema %q. Allowed forms: {\"schema\":%q,\"action\":\"refresh\"} or {\"schema\":%q,\"action\":\"noop\",\"reason\":\"one short sentence\"}. Never return a command, path, container name, script, or extra field.\n",
		persona, goal, actionSchema, actionSchema, actionSchema)
}
