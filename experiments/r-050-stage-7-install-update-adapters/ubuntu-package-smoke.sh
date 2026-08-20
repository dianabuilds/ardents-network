#!/bin/sh

set -eu
export DEBIAN_FRONTEND=noninteractive

scratch=${1:-}
case "$scratch" in
	/r050/package-smoke-*) ;;
	*)
		echo "scratch must be a new /r050/package-smoke-* path" >&2
		exit 64
		;;
esac
if [ -e "$scratch" ]; then
	echo "scratch already exists: $scratch" >&2
	exit 65
fi

umask 077
package_root="$scratch/package-root"
repository="$scratch/repository"
gnupg_home="$scratch/gnupg"
lists="$scratch/apt-lists"
sources="$scratch/ardents.sources"
keyring="$scratch/ardents-r050-archive.asc"
package="$repository/pool/main/a/ardents-bootstrap/ardents-bootstrap_0.0.0+r050.1_amd64.deb"

mkdir -p "$package_root/DEBIAN" "$package_root/usr/lib/ardents/bootstrap" \
	"$package_root/usr/bin" "$(dirname "$package")" \
	"$repository/dists/stable/main/binary-amd64" "$gnupg_home" \
	"$lists/partial"
chmod 0755 "$package_root/DEBIAN"
find "$package_root" -type d -exec chmod 0755 {} +

printf '%s\n' \
	'Package: ardents-bootstrap' \
	'Version: 0.0.0+r050.1' \
	'Section: net' \
	'Priority: optional' \
	'Architecture: amd64' \
	'Maintainer: Ardents R-050 experiment <invalid@example.invalid>' \
	'Description: disposable Stage 7 bootstrap package fixture' \
	>"$package_root/DEBIAN/control"

printf '%s\n' \
	'#!/bin/sh' \
	'echo "ardents-r050-bootstrap-fixture"' \
	>"$package_root/usr/lib/ardents/bootstrap/ardents-bootstrap"
printf '%s\n' \
	'#!/bin/sh' \
	'exec /usr/lib/ardents/bootstrap/ardents-bootstrap "$@"' \
	>"$package_root/usr/bin/ardents"
chmod 0755 "$package_root/usr/lib/ardents/bootstrap/ardents-bootstrap" \
	"$package_root/usr/bin/ardents"

source_date_epoch=1787184000
find "$package_root" -exec touch -h -d "@$source_date_epoch" {} +
SOURCE_DATE_EPOCH=$source_date_epoch dpkg-deb --build --root-owner-group \
	"$package_root" "$package" >/dev/null
first_package_hash=$(sha256sum "$package" | awk '{ print $1 }')
SOURCE_DATE_EPOCH=$source_date_epoch dpkg-deb --build --root-owner-group \
	"$package_root" "$package" >/dev/null
second_package_hash=$(sha256sum "$package" | awk '{ print $1 }')
test "$first_package_hash" = "$second_package_hash"
control_members=$(dpkg-deb --ctrl-tarfile "$package" | tar -tf -)
control_count=$(printf '%s\n' "$control_members" | grep -c '^\./control$')
unexpected_control=$(printf '%s\n' "$control_members" | grep -Ev '^(\./|\./control)$' || true)
if [ "$control_count" -ne 1 ] || [ -n "$unexpected_control" ]; then
	echo "unexpected control archive members:" >&2
	printf '%s\n' "$control_members" >&2
	exit 66
fi
payload_members=$(dpkg-deb --fsys-tarfile "$package" | tar -tf -)
printf '%s\n' "$payload_members" | grep -qx './usr/bin/ardents'
printf '%s\n' "$payload_members" | grep -qx './usr/lib/ardents/bootstrap/ardents-bootstrap'

cd "$repository"
apt-ftparchive packages pool >dists/stable/main/binary-amd64/Packages
gzip -n -9 -c dists/stable/main/binary-amd64/Packages \
	>dists/stable/main/binary-amd64/Packages.gz
apt-ftparchive \
	-o APT::FTPArchive::Release::Suite=stable \
	-o APT::FTPArchive::Release::Codename=stable \
	release dists/stable >dists/stable/Release

chmod 0700 "$gnupg_home"
GNUPGHOME="$gnupg_home" gpg --batch --passphrase '' --quick-gen-key \
	'Ardents R-050 ephemeral archive <invalid@example.invalid>' ed25519 sign 1d \
	>/dev/null 2>&1
fingerprint=$(GNUPGHOME="$gnupg_home" gpg --batch --with-colons --list-secret-keys | \
	awk -F: '$1 == "fpr" { print $10; exit }')
test -n "$fingerprint"
GNUPGHOME="$gnupg_home" gpg --batch --armor --export "$fingerprint" >"$keyring"
GNUPGHOME="$gnupg_home" gpg --batch --yes --local-user "$fingerprint" \
	--clearsign --output dists/stable/InRelease dists/stable/Release

printf '%s\n' \
	'Types: deb' \
	"URIs: file:$repository" \
	'Suites: stable' \
	'Components: main' \
	"Signed-By: $keyring" \
	>"$sources"

apt_options="-o Dir::Etc::sourcelist=$sources -o Dir::Etc::sourceparts=- -o Dir::State::lists=$lists -o APT::Sandbox::User=root"
# Word splitting is intentional: each option above is one apt argument without
# whitespace in any generated path.
# shellcheck disable=SC2086
apt-get $apt_options update >/dev/null
# shellcheck disable=SC2086
apt-get $apt_options install -y ardents-bootstrap >/dev/null
test "$(/usr/bin/ardents)" = 'ardents-r050-bootstrap-fixture'
before=$(sha256sum /usr/bin/ardents /usr/lib/ardents/bootstrap/ardents-bootstrap)

# shellcheck disable=SC2086
apt-get $apt_options install --reinstall -y ardents-bootstrap >/dev/null
after=$(sha256sum /usr/bin/ardents /usr/lib/ardents/bootstrap/ardents-bootstrap)
test "$before" = "$after"

# shellcheck disable=SC2086
apt-get $apt_options remove -y ardents-bootstrap >/dev/null
test ! -e /usr/bin/ardents
test ! -e /usr/lib/ardents/bootstrap/ardents-bootstrap
test ! -e /etc/ardents
test ! -e /var/lib/ardents
test ! -e /etc/systemd/system/ardents.service
test ! -e /usr/lib/systemd/system/ardents.service

printf '%s\n' \
	"image_os=$(awk -F= '$1 == "PRETTY_NAME" { gsub(/"/, "", $2); print $2 }' /etc/os-release)" \
	"architecture=$(dpkg --print-architecture)" \
	"dpkg_deb=$(dpkg-deb --version | head -n 1)" \
	"apt=$(apt-get --version | head -n 1)" \
	"apt_ftparchive=$(apt-ftparchive --version | head -n 1)" \
	"gpg=$(gpg --version | head -n 1)" \
	"archive_fingerprint=$fingerprint" \
	"source_date_epoch=$source_date_epoch" \
	"package_sha256=$(sha256sum "$package" | awk '{ print $1 }')" \
	"control_members=$(printf '%s' "$control_members" | tr '\n' ',')" \
	'install=pass reinstall=pass remove=pass residue=pass'
