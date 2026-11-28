package plan

import "errors"

// validateSnapshotStorage admits only a distinct storage root and a finite
// byte reservation. Absence declares no staging storage for this Endpoint.
func validateSnapshotStorage(plan Headless) error {
	if plan.SnapshotRoot == "" && plan.SnapshotQuotaBytes == 0 {
		return nil
	}
	root := plan.SnapshotRoot
	if plan.Role != "" || plan.SnapshotQuotaBytes <= 0 || root == "/" || !unitPath(root) {
		return errors.New("invalid Endpoint snapshot storage declaration")
	}
	for _, other := range []string{
		plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.ApplicationTokenRoot,
		plan.PublicationRoot, plan.ServiceInstanceRoot, plan.TimeConfidenceFile, plan.NetworkSourcePlan,
		plan.ApplicationSocket, plan.AdministrationSocket,
		plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath,
		plan.PublisherPermission.RequestPath, plan.PublisherPermission.ResponsePath,
	} {
		if other != "" && pathsOverlap(root, other) {
			return errors.New("Endpoint snapshot storage overlaps another declared path")
		}
	}
	return nil
}
