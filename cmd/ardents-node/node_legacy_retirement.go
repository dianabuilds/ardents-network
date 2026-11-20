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
	errOldSourceProfileRetired      = errors.New("old Source profile is retired")
	errOldSourceInstalledRetired    = errors.New("old Source start-installed orchestration is retired; use the platform-led Source lifecycle (ardents-control source prepare-state, ardents-next platform initialize-trust/unit, systemctl start ardents-source)")
	errOldSourceInstallationRetired = errors.New("old Source installation transactions are retired; use the platform-led Source lifecycle (ardents-next platform check, source serve-retained, source repair-retained)")
	errOldNodeConfigRetired         = errors.New("old Node --config runtime is retired; use ardents-next node <role> --retained-config")
	errOldNodeDutyRetired           = errors.New("old Node duty reservation is retired")
	errOldIssuerRetired             = errors.New("old Node issuer initialize/serve is retired; use ardents-next node-identity/issuance/node issuer")
	errOldTransitIssuerRetired      = errors.New("old Transit issuer start is retired")
	errOldHostingRetired            = errors.New("old Node hosting initialize is retired; use ardents-next hosting initialize")
)
