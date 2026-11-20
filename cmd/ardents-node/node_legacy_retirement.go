package main

import "errors"

// The legacy generic Node runtime and its operator plan are retired. Each
// maintained role now uses its explicit successor composition through the
// canonical ardents-next commands (node <role> --retained-config, hosting
// initialize, node-identity.import / issuance.initialize-profile / node issuer).
// These refusals fire before any private configuration is read or any effect is
// attempted; they grant no runtime, adoption or reset right. ADR-0125 separates
// software delivery from infrastructure deployment and creates no new installer.
var (
	errOldSourceProfileRetired          = errors.New("old Source profile is retired")
	errOldSourceInstalledRetired        = errors.New("old Source start-installed orchestration is retired; use the platform-led Source lifecycle (ardents-control source prepare-state, ardents-next platform initialize-trust/unit, systemctl start ardents-source)")
	errOldSourceInstallationRetired     = errors.New("old Source installation transactions are retired; use the platform-led Source lifecycle (ardents-next platform check, source serve-retained, source repair-retained)")
	errOldForwardingInstalledRetired    = errors.New("old Forwarding start-installed orchestration is retired; use the platform-led Forwarding lifecycle (ardents-next node forwarding --retained-config, platform check/unit, systemctl start ardents-forwarding)")
	errOldForwardingInstallationRetired = errors.New("old Forwarding installation transactions are retired; use the platform-led Forwarding lifecycle (ardents-next platform check, node forwarding --retained-config, forwarding repair-retained)")
	errOldJoinInstalledRetired          = errors.New("old JOIN start-installed orchestration is retired; use the platform-led JOIN lifecycle (ardents-next node join --retained-config, platform check/unit, systemctl start ardents-join)")
	errOldJoinInstallationRetired       = errors.New("old JOIN installation transactions are retired; use the platform-led JOIN lifecycle (ardents-next platform check, node join --retained-config, join repair-retained)")
	errOldNodeConfigRetired             = errors.New("old Node --config runtime is retired; use ardents-next node <role> --retained-config")
	errOldIssuerRetired                 = errors.New("old Node issuer initialize/serve is retired; use ardents-next node-identity/issuance/node issuer")
	errOldHostingRetired                = errors.New("old Node hosting initialize is retired; use ardents-next hosting initialize")
)
