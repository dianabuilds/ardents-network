package endpoint

import (
	"encoding/json"
	"strings"
	"testing"
)

// InstallationRequest encodes the independent installed Endpoint request fixture.
func InstallationRequest(t testing.TB, bundle, pin, history string) []byte {
	t.Helper()
	raw := `{"schema":"ardents-endpoint-installation-request-v1","bundle_root":"/bundle","manifest_sha256":"PIN","installation_root":"/installation","release_floor_root":"/floors","reference_time":"2030-01-02T03:04:05Z","headless":{"role":"reader","text_token_root":"/tokens","reader_permission":{"request_path":"/permissions/request","response_path":"/permissions/response","maxima":[1,0,0]},"publisher_permission":{"request_path":"","response_path":"","maxima":[0,0,0]},"schema":"ardents-headless-runtime-v2","network_state_root":"/state","entry_state_root":"/entry","transit_acquisition_root":"","application_socket":"/socket/reader","administration_socket":"","publication_root":"","local_role_state_root":"/roles","time_confidence_file":"/clock","network_id":"NET","network_authorities":["KEY"],"network_threshold":1,"network_profile":"ardents-route-v3","closed_profile_authority":"KEY","broker_id":"BROKER","connection_principal":"PRINCIPAL","administration_principal":"","bytes_each_direction":0},"source":{"schema":"ardents-source-plan-v1","network_id":"NET","authority_public":["KEY"],"threshold":1,"clock_observed_at":"2030-01-02T03:04:05Z","clock_observation_file":"/clock","order_seed":"SEED","materialization_index":0,"refresh_interval_ms":1000,"runtime_profile":"ardents-route-v3","local_role_state_root":"/roles","client_certificate":"/credentials/client.pem","client_key":"/credentials/client.key","sources":[{"address":"source-a.example:443","server_name":"source-a.example","identity":"SOURCE_A","family":"a","endpoint_handle":"a","root_ca":"/credentials/a.pem","leaf_key_digest":"LEAF"},{"address":"source-b.example:443","server_name":"source-b.example","identity":"SOURCE_B","family":"b","endpoint_handle":"b","root_ca":"/credentials/b.pem","leaf_key_digest":"LEAF"}]}}`
	raw = strings.NewReplacer("NET", strings.Repeat("02", 32), "KEY", strings.Repeat("ab", 32), "BROKER", strings.Repeat("03", 32), "PRINCIPAL", strings.Repeat("04", 32), "SEED", strings.Repeat("05", 32), "SOURCE_A", strings.Repeat("06", 32), "SOURCE_B", strings.Repeat("07", 32), "LEAF", strings.Repeat("08", 32)).Replace(raw)
	quoted := func(value string) string {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	raw = strings.Replace(raw, "\"PIN\"", quoted(pin), 1)
	raw = strings.Replace(raw, "\"bundle_root\":\"/bundle\"", "\"bundle_root\":"+quoted(bundle), 1)
	raw = strings.Replace(raw, "\"release_floor_root\":\"/floors\"", "\"release_floor_root\":"+quoted(history), 1)
	return []byte(raw + "\n")
}
