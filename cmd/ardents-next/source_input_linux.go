//go:build linux

package main

import "github.com/dianabuilds/ardents-network/internal/successor/network/source"

// sourceServePlan declares only distribution. State owns current evidence and
// local serving guards; this command cannot acquire or refresh an Epoch.
type sourceServePlan struct {
	Network           *networkAuthorityPlan `json:"network"`
	ProcessProfile    string                `json:"process_profile"`
	ParticipationRoot string                `json:"participation_root"`
	Address           string                `json:"address"`
	Certificate       string                `json:"certificate"`
	PrivateKey        string                `json:"private_key"`
	ClientRoot        string                `json:"client_root"`
	ClientPins        [][32]byte            `json:"client_pins"`
}

// Go's fixed-array decoder accepts short and overlong arrays. This closed plan
// checks exact shapes before those decoded values can become trust declarations.
func decodeSourceServePlan(raw []byte) (sourceServePlan, error) {
	d, err := source.DecodeServingDeclaration(raw)
	if err != nil {
		return sourceServePlan{}, err
	}
	return sourceServePlan{Network: (*networkAuthorityPlan)(d.Network), ProcessProfile: d.ProcessProfile, ParticipationRoot: d.ParticipationRoot, Address: d.Address, Certificate: d.Certificate, PrivateKey: d.PrivateKey, ClientRoot: d.ClientRoot, ClientPins: d.ClientPins}, nil
}
