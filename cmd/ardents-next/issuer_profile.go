package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

func runIssuerProfile(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	r := issuanceResult{Operation: operation, Phase: "input", Outcome: "invalid-input"}
	write := func() int {
		code := 1
		switch r.Outcome {
		case "identity-imported", "profile-initialized", "profile-verified", "binding-prepared":
			code = 0
		case "invalid-input":
			code = 2
		case "canceled":
			code = 130
		}
		if json.NewEncoder(out).Encode(r) != nil {
			return 2
		}
		return code
	}
	if !issuance.Supported() {
		r.Outcome = "unsupported-platform"
		return write()
	}
	if ctx == nil || len(args) < 2 || len(args) > 3 || args[0] != "--config" {
		return write()
	}
	if ctx.Err() != nil {
		r.Outcome = "canceled"
		return write()
	}
	collector := ""
	if len(args) == 3 {
		collector = args[2]
	}
	if collectorEndpoint(collector) != nil {
		return write()
	}
	raw, err := readBounded(args[1], 16<<10)
	if err != nil {
		return write()
	}
	defer clear(raw)
	observation, _ := newObservation(collector)
	finish := observation.beginIssuance(operation)
	var output string
	var bytes []byte
	switch operation {
	case "node-identity.import":
		fields, e := profileConfigObject(raw, "root", "binding", "key_file")
		err = e
		var root, source string
		var binding nodeidentity.Binding
		if err == nil {
			root, err = profilePath(fields["root"])
		}
		if err == nil {
			source, err = profilePath(fields["key_file"])
		}
		if err == nil {
			binding, err = decodeIdentityBinding(fields["binding"])
		}
		if err == nil {
			r.Phase = "import"
			err = nodeidentity.Import(ctx, root, source, binding)
			r.Outcome = "identity-imported"
		}
	case "issuance.initialize-profile", "issuance.inspect-profile":
		var p issuer.ProfilePlan
		p, output, err = decodeProfilePlan(raw)
		if err == nil {
			var result issuer.Result
			if operation == "issuance.initialize-profile" {
				result = issuer.InitializeProfile(ctx, p)
			} else {
				result = issuer.InspectProfile(ctx, p)
			}
			r.Phase, r.Outcome = result.Status()
			bytes = result.Response
		}
	case "admission.prepare-binding":
		output, bytes, err = prepareProfileBinding(raw)
		if err == nil {
			r.Phase = "prepare-binding"
			r.Outcome = "binding-prepared"
		}
	default:
		err = issuance.ErrInvalid
	}
	if err != nil {
		r.Outcome = issuanceOutcome(err)
	}
	if err == nil && bytes != nil {
		r.Phase = "export"
		err = exportIssuerProfile(ctx, output, bytes)
		if err != nil {
			r.Outcome = issuanceOutcome(err)
		}
	}
	finish(r.Phase, r.Outcome)
	_ = observation.close()
	_ = json.NewEncoder(diagnostic).Encode(r)
	return write()
}
