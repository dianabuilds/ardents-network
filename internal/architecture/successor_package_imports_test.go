package architecture

import "strings"

func isMigratedDomain(name string) bool {
	for _, root := range []string{
		"internal/domain/admission", "internal/canonicaldigest", "internal/domain/connection", "internal/domain/custody", "internal/durablefile", "internal/domain/endpoint",
		"internal/domain/enrollment", "internal/domain/execution", "internal/domain/hosting", "internal/domain/network", "internal/domain/node",
		"internal/pathcheck", "internal/platform/artifacts", "internal/platform/cgroup", "internal/platform/process", "internal/platform/systemd", "internal/platform/processguard", "internal/domain/publication",
		"internal/domain/reachability", "internal/domain/release", "internal/domain/route",
	} {
		if name == root || strings.HasPrefix(name, root+"/") {
			return true
		}
	}
	return false
}

// Match exact Go packages. A parent folder grants no child or sibling imports.
var successorPackageImports = map[string][]string{
	"internal/platform/processguard":                      {},
	"internal/canonicaldigest":                            {},
	"internal/durablefile":                                {},
	"internal/pathcheck":                                  {},
	"internal/domain/node/admission":                      {"domain/admission", "domain/admission/receiving", "domain/hosting", "domain/network", "domain/route/ardp", "domain/route/receiver"},
	"internal/domain/node/plan":                           {"domain/hosting", "pathcheck", "platform/processguard"},
	"internal/domain/node/receive":                        {"domain/admission", "domain/admission/issuer", "domain/admission/quota", "domain/admission/receiving", "domain/hosting", "domain/network", "domain/network/state", "domain/node/admission", "domain/reachability", "domain/route/ardp", "domain/route/channel", "domain/route/introduction", "domain/route/receiver", "domain/route/role", "pathcheck", "platform/artifacts"},
	"internal/domain/node/runtime":                        {"domain/admission", "domain/admission/issuer", "domain/admission/quota", "domain/admission/receiving", "domain/hosting", "domain/network", "domain/network/duty", "domain/network/state", "domain/node/admission", "domain/node/plan", "domain/reachability", "domain/route/ardp", "domain/route/bootstrap", "domain/route/introduction", "domain/route/receiver", "domain/route/role", "durablefile", "pathcheck", "platform/processguard"},
	"internal/domain/network":                             {},
	"internal/domain/network/canonicalcursor":             {},
	"internal/domain/network/closedprofile":               {"domain/network/canonicalcursor"},
	"internal/domain/network/duty":                        {"durablefile", "pathcheck"},
	"internal/domain/network/epoch":                       {"domain/network/canonicalcursor"},
	"internal/domain/network/source":                      {"domain/network/canonicalcursor", "domain/network/epoch"},
	"internal/domain/network/source/serving":              {"domain/network/source/servingdeclaration", "domain/network/state", "pathcheck", "platform/artifacts", "platform/processguard"},
	"internal/domain/network/source/servingdeclaration":   {},
	"internal/domain/network/state":                       {"canonicaldigest", "domain/network/closedprofile", "domain/network/duty", "domain/network/epoch", "domain/network/source", "domain/network/source/servingdeclaration", "domain/network/state/durable", "pathcheck"},
	"internal/domain/network/state/internal/distribution": {"domain/network/canonicalcursor"},
	"internal/domain/network/state/durable":               {"durablefile", "pathcheck"},
	"internal/domain/admission":                           {"domain/admission/issuerprofile"},
	"internal/domain/admission/issuerprofile":             {},
	"internal/domain/admission/allocation":                {"domain/admission"},
	"internal/domain/custody":                             {"domain/admission", "domain/admission/allocation", "domain/publication"},
	"internal/domain/admission/attempts":                  {"pathcheck"},
	"internal/domain/admission/spending":                  {"pathcheck"},
	"internal/domain/admission/token":                     {"domain/admission", "domain/admission/issuerprofile"},
	"internal/domain/admission/receiving":                 {"domain/admission", "domain/admission/token", "domain/admission/spending", "platform/artifacts"},
	"internal/domain/admission/stock":                     {"domain/admission", "domain/admission/allocation", "domain/admission/attempts", "domain/admission/token"},
	"internal/domain/admission/quota":                     {"domain/admission", "domain/admission/issuerprofile", "pathcheck"},
	"internal/domain/admission/issuance":                  {"canonicaldigest", "domain/admission", "domain/admission/quota", "domain/admission/issuerprofile", "domain/admission/issuerprofile/signing", "pathcheck"},
	"internal/domain/admission/operator":                  {"domain/admission", "domain/admission/issuance", "domain/admission/issuer", "domain/admission/quota", "platform/artifacts"},
	"internal/domain/admission/issuer":                    {"domain/admission", "domain/admission/quota", "domain/admission/issuance", "domain/admission/issuerprofile/signing", "pathcheck"},
	"internal/domain/admission/issuerprofile/signing":     {"domain/admission/issuerprofile", "pathcheck"},
	"internal/domain/hosting":                             {"canonicaldigest", "domain/hosting/counters", "pathcheck"},
	"internal/domain/hosting/counters":                    {},
	"internal/domain/publication":                         {},
	"internal/domain/publication/durable":                 {"domain/publication"},
	"internal/domain/publication/instance":                {"canonicaldigest", "domain/publication", "domain/publication/durable", "domain/reachability", "domain/route/introduction", "domain/route/capsule", "domain/route/join", "domain/connection/wire", "domain/execution/runtime"},
	"internal/domain/publication/runtime":                 {"domain/connection", "domain/execution/runtime", "domain/network", "domain/publication", "domain/publication/instance", "domain/reachability", "domain/route", "domain/route/introduction", "domain/route/prefix", "domain/route/capsule", "domain/route/join"},
	"internal/domain/connection":                          {"domain/execution/runtime", "domain/publication", "domain/publication/instance", "domain/route/capsule", "domain/route/join", "domain/connection/wire"},
	"internal/domain/connection/transfer":                 {"domain/connection/wire"},
	"internal/domain/connection/wire":                     {},
	"internal/domain/reachability":                        {"domain/publication", "pathcheck"},
	"internal/domain/enrollment":                          {"canonicaldigest", "pathcheck"},
	"internal/domain/release":                             {"canonicaldigest", "durablefile", "pathcheck"},
	"internal/domain/execution":                           {},
	"internal/domain/execution/runtime":                   {"domain/execution", "domain/execution/worker"},
	"internal/domain/execution/worker":                    {"domain/endpoint/unit", "canonicaldigest", "pathcheck"},
	"internal/domain/release/endpoint":                    {"domain/enrollment", "domain/release"},
	"internal/domain/release/infrastructure":              {"canonicaldigest", "domain/enrollment", "domain/release", "pathcheck"},
	"internal/domain/release/infrastructure/launch":       {"domain/admission/issuance", "canonicaldigest", "domain/node/plan", "domain/enrollment", "domain/network/source/serving", "domain/network/source/servingdeclaration", "domain/network/state", "pathcheck", "platform/artifacts", "platform/process", "platform/systemd", "domain/release", "domain/release/infrastructure"},
	"internal/domain/endpoint/launch":                     {"domain/release/endpoint", "canonicaldigest", "durablefile", "domain/enrollment", "domain/release", "platform/cgroup", "platform/systemd", "platform/process", "domain/endpoint/generation", "domain/endpoint/unit", "domain/endpoint/plan"},
	"internal/platform/cgroup":                            {"pathcheck"},
	"internal/platform/artifacts":                         {"durablefile", "pathcheck"},
	"internal/domain/endpoint/plan":                       {"canonicaldigest", "durablefile"},

	"internal/platform/systemd":           {},
	"internal/platform/process":           {"canonicaldigest", "pathcheck"},
	"internal/domain/endpoint/generation": {"canonicaldigest", "durablefile", "pathcheck"},
	"internal/domain/endpoint/unit":       {"canonicaldigest"},
	"internal/domain/route":               {},
	"internal/domain/route/ardp":          {},
	"internal/domain/route/issuer":        {"domain/route/ardp", "domain/admission"},
	"internal/domain/route/transport":     {},
	"internal/domain/route/channel":       {"domain/route/ardp", "domain/route/transport"},
	"internal/domain/route/bootstrap":     {"domain/route/ardp", "domain/route/channel"},
	"internal/domain/route/transport/tls": {"domain/route/transport"},

	"internal/domain/route/introduction": {"domain/admission/spending", "durablefile", "domain/route", "domain/route/ardp", "domain/route/role", "domain/route/prefix", "domain/route/capsule", "domain/network", "domain/admission", "pathcheck"},
	"internal/domain/route/capsule":      {},
	"internal/domain/route/selection":    {"domain/route", "domain/network", "durablefile"},
	"internal/domain/route/role":         {"domain/route", "domain/route/ardp", "domain/route/channel", "domain/route/transport", "domain/network", "domain/admission"},
	"internal/domain/route/join":         {"domain/route/prefix", "domain/route", "domain/route/selection", "domain/route/ardp", "domain/route/channel", "domain/route/role", "domain/route/transport", "domain/network", "domain/admission"},
	"internal/domain/route/join/pairing": {"domain/route/ardp", "domain/route/channel", "domain/route/role"},
	"internal/domain/route/prefix":       {"domain/route/issuer", "domain/route", "domain/route/selection", "domain/route/ardp", "domain/route/bootstrap", "domain/route/channel", "domain/route/role", "domain/route/transport", "domain/route/transport/tls", "domain/network", "domain/admission"},
	"internal/domain/route/receiver":     {"domain/route/issuer", "domain/route/join/pairing", "domain/route", "domain/route/role", "domain/route/transport", "domain/route/transport/tls", "domain/route/channel", "domain/route/bootstrap", "domain/route/ardp", "domain/route/introduction", "domain/network", "domain/admission", "domain/admission/receiving"},
	"internal/domain/endpoint/session":   {"domain/admission", "domain/admission/stock", "domain/endpoint/funding", "domain/endpoint/publisher", "domain/execution", "domain/execution/runtime", "domain/hosting", "domain/reachability", "domain/route/ardp", "domain/route/bootstrap", "domain/route/capsule", "domain/route/channel", "domain/route/introduction", "domain/route/join", "domain/route/prefix", "domain/route/role", "domain/route/selection", "pathcheck", "platform/artifacts"},
	"internal/domain/endpoint":           {"domain/endpoint/funding", "domain/endpoint/reader", "domain/endpoint/publisher", "domain/admission", "domain/admission/stock", "canonicaldigest", "domain/execution", "domain/execution/runtime", "domain/hosting", "domain/endpoint/launch", "domain/network", "domain/network/source", "domain/network/state", "pathcheck"},
	"internal/domain/endpoint/publisher": {"domain/endpoint/funding", "domain/admission", "domain/admission/stock", "domain/execution/runtime", "domain/hosting", "domain/network", "domain/publication/durable", "domain/publication/instance", "domain/publication/runtime", "domain/route/prefix", "domain/route/selection", "pathcheck"},
	"internal/domain/endpoint/reader":    {"domain/endpoint/funding", "domain/admission", "domain/admission/stock", "domain/connection", "domain/execution/runtime", "domain/hosting", "domain/network", "domain/reachability", "domain/route/prefix", "domain/route/selection"},
	"internal/domain/endpoint/funding":   {"domain/admission", "domain/admission/stock", "domain/admission/token", "domain/hosting", "domain/network", "domain/network/state", "domain/route", "domain/route/ardp", "domain/route/bootstrap", "domain/route/prefix", "domain/route/role", "domain/route/selection", "pathcheck"},
	"cmd/ardents-next":                   {"canonicaldigest", "pathcheck", "domain/endpoint/session", "domain/endpoint/publisher", "domain/endpoint/reader", "domain/endpoint/funding", "domain/route/introduction", "domain/route/prefix", "domain/network", "domain/network/state", "domain/admission/stock", "domain/admission/receiving", "domain/admission/allocation", "domain/admission", "domain/admission/quota", "domain/admission/issuerprofile", "domain/admission/issuance", "domain/admission/issuer", "domain/admission/issuerprofile/signing", "domain/hosting", "domain/route", "domain/route/role", "domain/route/selection", "domain/route/join", "domain/route/receiver", "domain/route/channel", "domain/route/ardp"},

	"internal/domain/endpoint/internal/permission": {"domain/admission/stock", "pathcheck"},
}

// Apply the exact package edges after the special file-scoped rules.
func successorDomainPackageImport(source, dependency, owner string) bool {
	if (source == "internal/domain/endpoint/history_preparation_linux.go" || source == "internal/domain/endpoint/history_preparation_linux_test.go") && dependency == modulePath+"/internal/domain/route/selection" {
		return true
	}
	if (source == "internal/domain/endpoint/history_preparation_linux.go" || source == "internal/domain/endpoint/history_preparation_linux_test.go") && dependency == modulePath+"/internal/domain/network/duty" {
		return true
	}
	if (source == "internal/domain/endpoint/source_trust_native_linux_test.go") && dependency == modulePath+"/internal/platform/artifacts" {
		return true
	}
	if source == "internal/domain/endpoint/history_preparation_linux_test.go" && dependency == modulePath+"/internal/domain/admission/attempts" {
		return true
	}

	if source == "internal/domain/network/source/command/serve_linux.go" && dependency == modulePath+"/internal/platform/processguard" {
		return true
	}
	if dependency == modulePath+"/internal/platform/artifacts" {
		switch source {
		case "cmd/ardents-next/inputs.go", "cmd/ardents-next/publication.go", "cmd/ardents-next/route_linux.go",
			"internal/domain/network/source/command/serve_linux.go", "internal/domain/network/source/command/prepare_linux.go",
			"internal/domain/network/state/source_state_files_linux_test.go":
			return true
		}
	}

	if source == "cmd/ardents-next/node_roles.go" && (dependency == modulePath+"/internal/platform/processguard") {
		return true
	}
	if (source == "internal/domain/node/runtime/start.go" || source == "internal/domain/node/runtime/start_other.go") && dependency == modulePath+"/internal/domain/admission/issuer" {
		return true
	}
	if source == "internal/domain/node/runtime/issuer_unix.go" && (dependency == modulePath+"/internal/domain/admission/issuer" || dependency == modulePath+"/internal/domain/admission/quota" || dependency == modulePath+"/internal/domain/route/bootstrap") {
		return true
	}
	if source == "internal/domain/node/runtime/introduction_unix.go" && dependency == modulePath+"/internal/domain/route/introduction" {
		return true
	}
	if source == "internal/domain/node/runtime/descriptor_unix.go" && dependency == modulePath+"/internal/domain/reachability" {
		return true
	}
	if source == "internal/domain/endpoint/document_command_installed_linux_test.go" {
		switch dependency {
		case modulePath + "/internal/domain/execution",
			modulePath + "/internal/domain/execution/runtime",
			modulePath + "/internal/domain/publication",
			modulePath + "/internal/domain/reachability",
			modulePath + "/internal/domain/route/transport":
			return true
		}
	}
	if source == "internal/domain/endpoint/connection_accepted_request_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/execution/worker" || dependency == modulePath+"/internal/domain/route/transport") {
		return true
	}
	if source == "internal/domain/endpoint/connection_response_loss_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/publication/runtime" || dependency == modulePath+"/internal/domain/route/transport") {
		return true
	}
	if source == "internal/domain/publication/runtime/publisher_streams.go" && dependency == modulePath+"/internal/domain/connection/wire" {
		return true
	}
	if source == "internal/domain/endpoint/connection_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/publication/runtime" || dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/domain/route/transport") {
		return true
	}

	if source == "internal/domain/endpoint/connection_authority_linux_test.go" && (dependency == modulePath+"/internal/domain/connection" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/route/transport") {
		return true
	}
	if source == "internal/domain/endpoint/publication_ack_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/execution/runtime" || dependency == modulePath+"/internal/domain/publication" || dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/domain/route/transport") {
		return true
	}
	if source == "internal/domain/endpoint/installed_service_linux_test.go" && (dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/domain/route/transport") {
		return true
	}
	if source == "internal/domain/endpoint/publication_installed_linux_test.go" && (dependency == modulePath+"/internal/domain/publication" || dependency == modulePath+"/internal/domain/publication/instance" || dependency == modulePath+"/internal/domain/reachability" || dependency == modulePath+"/internal/domain/route/transport" || dependency == modulePath+"/internal/domain/route/capsule") {
		return true
	}

	if source == "cmd/ardents-next/route_linux.go" && dependency == modulePath+"/internal/domain/route/capsule" {
		return true
	}
	if source == "cmd/ardents-next/publication.go" && (dependency == modulePath+"/internal/domain/publication/durable" || dependency == modulePath+"/internal/domain/publication/instance") {
		return true
	}
	if source == "internal/domain/endpoint/participant_linux.go" && (dependency == modulePath+"/internal/domain/endpoint/launch" || dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/execution/runtime") {
		return true
	}
	if source == "cmd/ardents-next/node.go" && dependency == modulePath+"/internal/domain/node/provisioning/runtimeplan" {
		return true
	}
	if source == "internal/domain/endpoint/declarations_linux.go" && (dependency == modulePath+"/internal/domain/execution" || dependency == modulePath+"/internal/domain/network/source") {
		return true
	}
	if (source == "cmd/ardents-next/execution_holder.go" || source == "cmd/ardents-next/admission_holder.go") && dependency == modulePath+"/internal/domain/execution/runtime" {
		return true
	}
	if source == "cmd/ardents-next/execution_holder.go" && dependency == modulePath+"/internal/domain/execution" {
		return true
	}

	if source == "cmd/ardents-next/infrastructure_linux.go" {
		return dependency == modulePath+"/internal/domain/release/infrastructure" || dependency == modulePath+"/internal/domain/release" || dependency == modulePath+"/internal/platform/artifacts"
	}
	if dependency == modulePath+"/internal/domain/enrollment" && source == "cmd/ardents-next/enrollment.go" {
		return true
	}

	if dependency == modulePath+"/internal/domain/route/transport" {
		if source == "internal/domain/endpoint/execution_installed_linux_test.go" {
			return true
		}
		if source == "internal/domain/endpoint/route_issuer_fixture_linux_test.go" {
			return true
		}
		switch source {
		case "internal/domain/endpoint/route_fixture_linux_test.go":
			return true
		}
	}
	if source == "internal/domain/endpoint/network_admission_fixture_linux_test.go" && dependency == modulePath+"/internal/domain/network/epoch" {
		return true
	}
	if source == "cmd/ardents-next/route_issuer_linux.go" && dependency == modulePath+"/internal/domain/route/bootstrap" {
		return true
	}
	if dependency == modulePath+"/internal/domain/reachability" && (source == "internal/domain/route/prefix/descriptor_exchange.go" || source == "internal/domain/route/receiver/receiver_unix.go" || source == "internal/domain/route/receiver/descriptor_unix.go" || source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route.go" || source == "cmd/ardents-next/admission_holder.go") {
		return true
	}
	if (source == "internal/domain/admission/stock/issuance_fixture_test.go" || source == "internal/domain/admission/stock/observation_retirement_linux_test.go" || source == "internal/domain/admission/stock/lifecycle_test.go" || source == "internal/domain/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/domain/admission/issuer" || dependency == modulePath+"/internal/domain/admission/issuance" || dependency == modulePath+"/internal/domain/admission/quota") {
		return true
	}
	if (source == "internal/domain/admission/stock/cycle_test.go" || source == "internal/domain/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/domain/admission/receiving" || dependency == modulePath+"/internal/domain/admission/spending") {
		return true
	}
	if source == "internal/domain/connection/publisher_recovery.go" && dependency == modulePath+"/internal/domain/route/prefix" {
		return true
	}
	if source == "internal/domain/connection/initiator_source.go" {
		for _, owned := range []string{"domain/network", "domain/reachability", "domain/route", "domain/route/prefix", "domain/route/selection", "domain/route/introduction"} {
			if dependency == modulePath+"/internal/"+owned {
				return true
			}
		}
	}
	// The Service TLS owners borrow only the closed key-exchange group list
	// from the Route TLS contract (#903). Peer identity, certificate and
	// proof checks stay with each Service owner; no other file gains the edge.
	if dependency == modulePath+"/internal/domain/route/transport" {
		switch source {
		case "internal/domain/connection/initiator_authentication.go", "internal/domain/publication/instance/service_attachment.go":
			return true
		}
	}
	if dependency == modulePath+"/internal/domain/network/source/servingdeclaration" &&
		source == "internal/domain/network/state/source_state_files_linux_test.go" {
		return true
	}

	// A registered package's external behavior tests may import that package.
	// This grants neither another package nor nested tests its imports.
	if _, registered := successorPackageImports[owner]; registered && strings.HasSuffix(source, "_test.go") && dependency == modulePath+"/"+owner {
		return true
	}
	for _, allowed := range successorPackageImports[owner] {
		if dependency == modulePath+"/internal/"+allowed {
			return true
		}
	}
	return false
}
