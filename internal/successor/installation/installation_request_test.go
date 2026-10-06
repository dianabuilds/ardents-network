package installation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Independent exact schema bytes: this fixture never calls the production
// encoder or supplies successful Network, Release or filesystem authority.
func requestFixture() []byte {
	raw := `{"schema":"ardents-endpoint-installation-request-v1","bundle_root":"/bundle","manifest_sha256":"PIN","installation_root":"/installation","release_floor_root":"/floors","reference_time":"2030-01-02T03:04:05Z","headless":{"role":"reader","text_token_root":"/tokens","reader_permission":{"request_path":"/permissions/request","response_path":"/permissions/response","maxima":[1,0,0]},"publisher_permission":{"request_path":"","response_path":"","maxima":[0,0,0]},"schema":"ardents-headless-runtime-v2","network_state_root":"/state","entry_state_root":"/entry","transit_acquisition_root":"","application_socket":"/socket/reader","administration_socket":"","publication_root":"","local_role_state_root":"/roles","time_confidence_file":"/clock","network_id":"NET","network_authorities":["KEY"],"network_threshold":1,"network_profile":"ardents-route-v3","closed_profile_authority":"KEY","broker_id":"BROKER","connection_principal":"PRINCIPAL","administration_principal":"","bytes_each_direction":0},"source":{"schema":"ardents-source-plan-v1","network_id":"NET","authority_public":["KEY"],"threshold":1,"clock_observed_at":"2030-01-02T03:04:05Z","clock_observation_file":"/clock","order_seed":"SEED","materialization_index":0,"refresh_interval_ms":1000,"runtime_profile":"ardents-route-v3","local_role_state_root":"/roles","client_certificate":"/credentials/client.pem","client_key":"/credentials/client.key","sources":[{"address":"source-a.example:443","server_name":"source-a.example","identity":"SOURCE_A","family":"a","endpoint_handle":"a","root_ca":"/credentials/a.pem","leaf_key_digest":"LEAF"},{"address":"source-b.example:443","server_name":"source-b.example","identity":"SOURCE_B","family":"b","endpoint_handle":"b","root_ca":"/credentials/b.pem","leaf_key_digest":"LEAF"}]}}`
	raw = strings.NewReplacer("PIN", strings.Repeat("01", 32), "NET", strings.Repeat("02", 32), "KEY", strings.Repeat("ab", 32), "BROKER", strings.Repeat("03", 32), "PRINCIPAL", strings.Repeat("04", 32), "SEED", strings.Repeat("05", 32), "SOURCE_A", strings.Repeat("06", 32), "SOURCE_B", strings.Repeat("07", 32), "LEAF", strings.Repeat("08", 32)).Replace(raw)
	return []byte(raw + "\n")
}

func TestInstallationRequestPortableAdmission(t *testing.T) {
	raw := requestFixture()
	r, err := DecodeRequest(t.Context(), raw, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.BundleRoot() != "/bundle" || r.ReleaseHistoryRoot() != "/floors" || r.ManifestSHA256() != strings.Repeat("01", 32) || r.ReferenceTime().Year() != 2030 {
		t.Fatal("checked request projection differs")
	}
	raw[0] = 'x'
	if r.BundleRoot() != "/bundle" {
		t.Fatal("request retained caller bytes")
	}
	withoutPin := strings.Replace(string(requestFixture()), `"manifest_sha256":"`+strings.Repeat("01", 32)+`",`, "", 1)
	if _, err := DecodeRequest(t.Context(), []byte(withoutPin), false); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRequest(t.Context(), []byte(withoutPin), true); !errors.Is(err, ErrInput) {
		t.Fatal("initial request accepted without independent pin")
	}
	if _, err := DecodeRequest(t.Context(), requestFixture(), false); !errors.Is(err, ErrInput) {
		t.Fatal("successor request accepted initial pin")
	}
	publisher := strings.NewReplacer(
		`"role":"reader",`, "",
		`"publisher_permission":{"request_path":"","response_path":"","maxima":[0,0,0]}`, `"publisher_permission":{"request_path":"/permissions/publisher-request","response_path":"/permissions/publisher-response","maxima":[128,0,0]}`,
		`"administration_socket":""`, `"administration_socket":"/socket/administration"`,
		`"publication_root":""`, `"publication_root":"/publication","service_instance_root":"/instance"`,
		`"administration_principal":""`, `"administration_principal":"`+strings.Repeat("09", 32)+`"`,
	).Replace(string(requestFixture()))
	if _, err := DecodeRequest(t.Context(), []byte(publisher), true); err != nil {
		t.Fatalf("ordinary participant declarations refused: %v", err)
	}
}

type requestHandoffCancellation struct {
	context.Context
	calls int
}

func (c *requestHandoffCancellation) Err() error {
	c.calls++
	if c.calls == 2 {
		return context.Canceled
	}
	return nil
}

func TestInstallationRequestOriginalCancellationAtHandoff(t *testing.T) {
	// A bounded CPU decode still owes its original caller a final handoff check.
	ctx := &requestHandoffCancellation{Context: context.Background()}
	r, err := DecodeRequest(ctx, requestFixture(), true)
	if !errors.Is(err, context.Canceled) || r.declared != nil {
		t.Fatal("late cancellation published checked declarations")
	}
}

func TestInstallationRequestCausalRefusals(t *testing.T) {
	key := strings.Repeat("ab", 32)
	for name, mutation := range map[string]func(string) string{
		"duplicate-field": func(s string) string {
			return strings.Replace(s, `"bundle_root":"/bundle"`, `"bundle_root":"/other","bundle_root":"/bundle"`, 1)
		},
		"unknown-field": func(s string) string { return strings.Replace(s, `"schema":`, `"extra":true,"schema":`, 1) },
		"duplicate-key": func(s string) string {
			return strings.Replace(s, `"network_authorities":["`+key+`"]`, `"network_authorities":["`+key+`","`+strings.ToUpper(key)+`"]`, 1)
		},
		"threshold":     func(s string) string { return strings.Replace(s, `"network_threshold":1`, `"network_threshold":2`, 1) },
		"shared-family": func(s string) string { return strings.Replace(s, `"family":"b"`, `"family":"a"`, 1) },
		"immutable-overlap": func(s string) string {
			return strings.Replace(s, `"installation_root":"/installation"`, `"installation_root":"/bundle/installation"`, 1)
		},
		"mutable-overlap": func(s string) string {
			return strings.Replace(s, `"entry_state_root":"/entry"`, `"entry_state_root":"/state/entry"`, 1)
		},
		"host-path": func(s string) string {
			return strings.Replace(s, `"bundle_root":"/bundle"`, `"bundle_root":"C:/bundle"`, 1)
		},
		"publisher-input": func(s string) string {
			return strings.Replace(s, `"publication_root":""`, `"publication_root":"/publisher"`, 1)
		},
		"clock-mismatch": func(s string) string {
			return strings.Replace(s, `"clock_observation_file":"/clock"`, `"clock_observation_file":"/other-clock"`, 1)
		},
		"alternate-json": func(s string) string { return " " + s },
		"oversized":      func(s string) string { return s + strings.Repeat(" ", 64<<10) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRequest(t.Context(), []byte(mutation(string(requestFixture()))), true); !errors.Is(err, ErrInput) {
				t.Fatalf("invalid declaration accepted: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, input := range []struct {
		name string
		ctx  context.Context
		want error
	}{{"original-cancellation", ctx, context.Canceled}, {"nil-context", nil, ErrInput}} {
		t.Run(input.name, func(t *testing.T) {
			if _, err := DecodeRequest(input.ctx, requestFixture(), true); !errors.Is(err, input.want) {
				t.Fatalf("invalid context admitted: %v", err)
			}
		})
	}
}
