package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

type endpointPlan struct {
	Role, NetworkID, BrokerID, AuthorityPublic, ConnectionPrincipal string
	AdministrationPrincipal, Target, IntroductionAcknowledgement    string
	ApplicationSocket, RouteSocket, AdministrationSocket            string
	PublicationFile, CredentialFile, InstanceKeyFile                string
	At, Deadline                                                    string
	BytesEachDirection                                              uint32
}

func readPlan(path string) (endpointPlan, error) {
	file, err := os.Open(path)
	if err != nil {
		return endpointPlan{}, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil || len(raw) == 0 || len(raw) > 64<<10 {
		return endpointPlan{}, errors.New("endpoint plan is empty or exceeds 64 KiB")
	}
	var value endpointPlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return endpointPlan{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return endpointPlan{}, errors.New("endpoint plan contains multiple JSON values")
	}
	if err := value.validate(); err != nil {
		return endpointPlan{}, err
	}
	return value, nil
}

func (value endpointPlan) validate() error {
	if value.Role != "client" && value.Role != "publisher" {
		return errors.New("endpoint role is invalid")
	}
	if value.ApplicationSocket == "" || value.RouteSocket == "" || value.PublicationFile == "" ||
		value.At == "" || value.Deadline == "" || value.BytesEachDirection == 0 || value.BytesEachDirection > 64<<10 {
		return errors.New("endpoint plan is incomplete or outside its bound")
	}
	if value.Role == "client" && (value.Target == "" || value.AdministrationSocket != "" ||
		value.CredentialFile != "" || value.InstanceKeyFile != "" || value.AdministrationPrincipal != "") {
		return errors.New("client plan contains publisher administration input")
	}
	if value.Role == "publisher" && (value.AdministrationSocket == "" || value.CredentialFile == "" ||
		value.InstanceKeyFile == "" || value.IntroductionAcknowledgement == "") {
		return errors.New("publisher plan lacks its administration input")
	}
	if _, err := time.Parse(time.RFC3339, value.At); err != nil {
		return err
	}
	deadline, err := time.ParseDuration(value.Deadline)
	if err != nil || deadline <= 0 || deadline > 15*time.Second {
		return errors.New("endpoint deadline is outside the frozen bound")
	}
	return nil
}

func fixedHex(value string, destination []byte) error {
	if len(value) != len(destination)*2 {
		return errors.New("hexadecimal field has wrong length")
	}
	_, err := hex.Decode(destination, []byte(value))
	return err
}
