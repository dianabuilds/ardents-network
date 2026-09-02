#!/bin/sh
set -eu

invalid() {
	printf 'service Credential response Linux profile invalid environment: %s\n' "$1" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || invalid "required command is unavailable: $1"
}

test_timeout=''
for argument in "$@"; do
	case $argument in
		-timeout=*) test_timeout=${argument#-timeout=} ;;
		*) invalid "unsupported runner argument: $argument" ;;
	esac
done
case $test_timeout in
	[1-9]*[smh]) ;;
	*) invalid 'test timeout must be a positive whole-second, minute, or hour duration' ;;
esac

require_command docker
require_command go
require_command id
require_command mktemp
require_command rm

user_id=$(id -u)
group_id=$(id -g)
case $user_id:$group_id in
	*[!0-9:]*|:*|*:) invalid 'invoking user and group identifiers must be numeric' ;;
esac
[ "$user_id" -ne 0 ] || invalid 'profile must run from an unprivileged account'
if ! docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
	invalid 'Docker daemon is unavailable to the invoking user'
fi
if ! docker image inspect golang:1.26.6 >/dev/null 2>&1; then
	invalid 'required golang:1.26.6 image is unavailable'
fi

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd -P)
temp_parent=${TMPDIR:-/tmp}
case $temp_parent in
	/*) ;;
	*) invalid 'TMPDIR must be an absolute POSIX path' ;;
esac
umask 077
scratch=$(mktemp -d "${temp_parent%/}/ardents-service-credential-response.XXXXXXXX") ||
	invalid 'cannot create an owned temporary artifact directory'

finish() {
	status=$?
	if [ -d "$scratch" ]; then
		case $scratch in
			"${temp_parent%/}"/ardents-service-credential-response.*)
				rm -rf -- "$scratch" || status=1
				;;
			*)
				printf 'service Credential response Linux profile refused to remove unexpected path: %s\n' "$scratch" >&2
				status=1
				;;
		esac
	fi
	exit "$status"
}
trap finish EXIT

cd "$repository_root"
export GOENV=off
export GOTOOLCHAIN=local
export GOFLAGS=-mod=readonly
export GOPROXY=off
export GOCACHE="$scratch/go-build"

if ! GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$scratch/ardents" ./cmd/ardents; then
	invalid 'Linux amd64 ardents cross-build failed; the selected Go toolchain or module cache is unavailable'
fi
if ! GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$scratch/ardents-custody" ./cmd/ardents-custody; then
	invalid 'Linux amd64 ardents-custody cross-build failed; the selected Go toolchain or module cache is unavailable'
fi
if ! GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -tags=service_credential_response_linux -trimpath -buildvcs=false -o "$scratch/service-credential-response.test" ./tests/e2e/service; then
	invalid 'Linux Credential response test cross-build failed; the selected Go toolchain or module cache is unavailable'
fi

docker_mount_source=$scratch
if command -v cygpath >/dev/null 2>&1; then
	docker_mount_source=$(cygpath -w "$scratch")
fi

MSYS_NO_PATHCONV=1 docker run --rm --platform linux/amd64 --network none --read-only --cap-drop ALL \
	--security-opt no-new-privileges --pids-limit 64 --memory 768m --cpus 1 \
	--user "$user_id:$group_id" --mount "type=bind,src=$docker_mount_source,dst=/work,readonly" \
	--tmpfs /tmp:rw,nosuid,nodev,size=512m,mode=1777 --workdir /work \
	-e TMPDIR=/tmp -e ARDENTS_E2E_PRODUCT_ARDENTS=/work/ardents \
	-e ARDENTS_E2E_PRODUCT_ARDENTS_CUSTODY=/work/ardents-custody \
	golang:1.26.6 /work/service-credential-response.test \
	-test.run '^TestLinuxCredentialResponsePublicationRecoversAfterFileSizeLimit$' -test.count=1 -test.v -test.timeout="$test_timeout"
