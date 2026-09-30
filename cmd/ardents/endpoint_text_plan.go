package main

import (
	"errors"
	"path/filepath"
)

func validateHeadlessTextFields(plan headlessRuntimePlan) error {
	if plan.TransitAcquisitionRoot != "" || plan.BytesEachDirection != 0 || plan.AlphaCorpusStateRoot != "" || plan.AlphaCorpusAuthority != "" || plan.AlphaCohort != "" {
		return errors.New("text runtime plan cannot select legacy acquisition or interfaces")
	}
	paths := []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot, plan.ApplicationSocket, plan.ReaderPermission.RequestPath, plan.ReaderPermission.ResponsePath}
	permissions := []headlessPermissionPlan{plan.ReaderPermission}
	if plan.Role == "reader" {
		if plan.PublicationRoot != "" || plan.ServiceInstanceRoot != "" || plan.AdministrationSocket != "" || plan.AdministrationPrincipal != "" || plan.PublisherPermission != (headlessPermissionPlan{}) {
			return errors.New("reader runtime cannot select Publisher inputs")
		}
	} else {
		paths = append(paths, plan.PublicationRoot, plan.ServiceInstanceRoot, plan.AdministrationSocket, plan.PublisherPermission.RequestPath, plan.PublisherPermission.ResponsePath)
		permissions = append(permissions, plan.PublisherPermission)
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || seen[path] {
			return errors.New("text runtime paths must be distinct, absolute and canonical")
		}
		seen[path] = true
	}
	for index, permission := range permissions {
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
