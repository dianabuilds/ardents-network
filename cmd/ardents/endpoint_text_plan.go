package main

import (
	"errors"
	"path/filepath"
)

func validateHeadlessTextFields(plan headlessRuntimePlan, protected bool) error {
	if !protected {
		if plan.TextTokenRoot != "" || plan.ReaderPermission != (headlessPermissionPlan{}) || plan.PublisherPermission != (headlessPermissionPlan{}) {
			return errors.New("text permission fields require runtime plan v2")
		}
		return nil
	}
	if plan.TransitAcquisitionRoot != "" || plan.BytesEachDirection != 0 || plan.AlphaCorpusStateRoot != "" || plan.AlphaCorpusAuthority != "" || plan.AlphaCohort != "" {
		return errors.New("text runtime plan cannot select legacy acquisition or interfaces")
	}
	seen := make(map[string]bool)
	for _, path := range []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot, plan.PublicationRoot, plan.ServiceInstanceRoot, plan.ApplicationSocket, plan.AdministrationSocket, plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath, plan.PublisherPermission.RequestPath, plan.PublisherPermission.ResponsePath} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || seen[path] {
			return errors.New("text runtime paths must be distinct, absolute and canonical")
		}
		seen[path] = true
	}
	for index, permission := range []headlessPermissionPlan{plan.ReaderPermission, plan.PublisherPermission} {
		maximum := uint64(4096)
		if index == 1 {
			maximum = 16384
		}
		total := uint64(permission.Maxima[0]) + uint64(permission.Maxima[1]) + uint64(permission.Maxima[2])
		if total == 0 || total > maximum {
			return errors.New("text runtime allocation unavailable")
		}
	}
	return nil
}
