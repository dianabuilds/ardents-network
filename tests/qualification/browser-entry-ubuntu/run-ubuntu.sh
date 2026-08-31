#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
    echo "usage: run-ubuntu.sh SIGNED_XPI ARTIFACT_DIRECTORY" >&2
    exit 2
fi

signed_xpi=$1
artifacts=$2
expected_xpi_sha256=d88e8ecba84cda82a7b2354d1f445e19b9d092f3f3d068868d1173ef29eaa2a2
platform=linux-amd64
endpoint_name=ardents-$platform
adapter_name=ardents-browser-$platform
host_name=ardents-browser-entry-$platform
control_name=ardents-control-$platform
extension_name=ardents-alpha-browser-entry.xpi
native_host_name=org.ardents.alpha_browser_entry

if [ "$(id -u)" -eq 0 ]; then
    echo "Browser Entry Ubuntu enrollment qualifier must run as an unprivileged user" >&2
    exit 2
fi
if [ ! -f "$signed_xpi" ] || [ ! -f "$artifacts/$endpoint_name" ] || [ ! -f "$artifacts/$adapter_name" ] || [ ! -f "$artifacts/$host_name" ] || [ ! -f "$artifacts/$control_name" ]; then
    echo "Browser Entry Ubuntu enrollment qualifier inputs are unavailable" >&2
    exit 2
fi
if [ "$(sha256sum "$signed_xpi" | awk '{print $1}')" != "$expected_xpi_sha256" ]; then
    echo "Browser Entry Ubuntu enrollment qualifier received a different signed XPI" >&2
    exit 2
fi

umask 077
scratch=$(mktemp -d "${TMPDIR:-/tmp}/ardents-browser-entry-v4.XXXXXX")
input=$scratch.enrollment.json
home_root=$scratch.home
bundle=$scratch.bundle
cleanup() {
    rm -f "$input"
    rm -rf "$scratch"
}
trap cleanup EXIT HUP INT TERM
mkdir -p "$home_root" "$bundle"
export HOME=$home_root

for name in "$endpoint_name" "$adapter_name" "$host_name" "$control_name"; do
    cp "$artifacts/$name" "$bundle/$name"
    chmod 700 "$bundle/$name"
done
cp "$signed_xpi" "$bundle/$extension_name"
chmod 600 "$bundle/$extension_name"

write_static() {
    printf '%s' "$2" > "$bundle/$1"
    chmod 600 "$bundle/$1"
}
descriptor="schema=ardents-closed-alpha-enrollment-v4
cohort=browser-entry-ubuntu-qualification
release=mozilla-signed-0.1.0
platform=$platform
environment=alpha
network=browser-entry-ubuntu-qualification
target_path=ardents/$platform/endpoint
artifact=$endpoint_name
trusted_root=1.root.json
control_catalog=catalog.ac1
disclosure_root=catalog.pub
control_release=release.ac1
control_network=network.ac1
control_compatibility=compatibility.ac1
control_release_root=release.pub
control_network_root=network.pub
control_compatibility_root=compatibility.pub
corpus_authority=corpus.pub
control_artifact=$control_name
browser_adapter_artifact=$adapter_name
browser_entry_artifact=$host_name
browser_entry_extension=$extension_name
"
write_static 1.root.json 'qualification trusted root'
write_static RELEASE "$descriptor"
write_static catalog.ac1 'qualification control catalog'
write_static catalog.pub 'qualification disclosure root'
write_static compatibility.ac1 'qualification compatibility decision'
write_static compatibility.pub 'qualification compatibility root'
write_static corpus.pub 'qualification corpus authority'
write_static network.ac1 'qualification network decision'
write_static network.pub 'qualification network root'
write_static release.ac1 'qualification release decision'
write_static release.pub 'qualification release root'
write_static timestamp.json 'qualification timestamp'

(
    cd "$bundle"
    find . -maxdepth 1 -type f ! -name SHA256SUMS -printf '%f\n' | LC_ALL=C sort | while IFS= read -r name; do
        sha256sum "$name"
    done
) > "$bundle/SHA256SUMS"
chmod 600 "$bundle/SHA256SUMS"
manifest_sha256=$(sha256sum "$bundle/SHA256SUMS" | awk '{print $1}')
printf '{"schema":"ardents-alpha-enrollment-input-v1","bundle_root":"%s","cohort":"browser-entry-ubuntu-qualification","release":"mozilla-signed-0.1.0","platform":"%s","manifest_sha256":"%s","environment":"alpha","network":"browser-entry-ubuntu-qualification","target_path":"ardents/%s/endpoint"}\n' "$bundle" "$platform" "$manifest_sha256" "$platform" > "$input"
chmod 600 "$input"

if ! install_result=$("$bundle/$host_name" install --enrollment "$input" --endpoint-artifact "$bundle/$endpoint_name" --at 2026-08-26T00:00:00Z); then
    echo 'Browser Entry Ubuntu enrollment manifest follows:' >&2
    sed -n '1,32p' "$bundle/SHA256SUMS" >&2
    echo 'Browser Entry Ubuntu enrollment descriptor follows:' >&2
    sed -n '1,32p' "$bundle/RELEASE" >&2
    exit 1
fi
native_manifest=$HOME/.mozilla/native-messaging-hosts/$native_host_name.json
if [ ! -f "$native_manifest" ] || ! printf '%s' "$install_result" | grep -F '"extension_installation":"manual-required"' >/dev/null ||
    ! grep -F "\"path\":\"$bundle/$host_name\"" "$native_manifest" >/dev/null ||
    ! grep -F '"alpha-browser-entry@ardents.network"' "$native_manifest" >/dev/null; then
    echo "Browser Entry Ubuntu enrollment installation did not create the exact native manifest" >&2
    exit 1
fi

remove_result=$("$bundle/$host_name" remove)
if [ -e "$native_manifest" ] || ! printf '%s' "$remove_result" | grep -F '"removal":"native-manifest-withdrawn"' >/dev/null; then
    echo "Browser Entry Ubuntu enrollment removal left the native manifest behind" >&2
    exit 1
fi
printf '%s\n' "{\"schema\":\"ardents-browser-entry-ubuntu-qualification-v1\",\"signed_xpi_sha256\":\"$expected_xpi_sha256\",\"native_manifest\":\"installed-and-withdrawn\",\"extension_installation\":\"manual-required\"}"
