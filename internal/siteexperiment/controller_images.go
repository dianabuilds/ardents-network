package siteexperiment

import "errors"

type experimentImages struct {
	application string
	tooling     string
	reference   string
}

func bindExperimentImages(application, tooling, reference string) (experimentImages, error) {
	images := experimentImages{application: application, tooling: tooling, reference: reference}
	if !validImageID(images.application) || !validImageID(images.tooling) || !validImageID(images.reference) {
		return experimentImages{}, errors.New("gate C requires three role-bound immutable image IDs")
	}
	return images, nil
}

func (images experimentImages) identities() []string {
	return []string{images.application, images.tooling, images.reference}
}
