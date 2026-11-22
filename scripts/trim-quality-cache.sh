#!/bin/sh
# Trim the shared quality build cache when it exceeds its disk budget.
# Invoked by `make quality-cache-trim`, which also runs at the end of a
# successful `make check`. Only the Go build cache is trimmed; the module
# cache, analyzer caches and compiled artifacts stay warm.
set -eu

if [ "$#" -ne 3 ]; then
	echo "usage: $0 <cache-root> <repository-root> <budget-gib>" >&2
	exit 2
fi

cache_root=$1
repository_root=$2
budget_gib=$3

case $cache_root in
/*) ;;
*)
	echo "trim-quality-cache: cache root must be absolute: $cache_root" >&2
	exit 2
	;;
esac
case $repository_root in
/*) ;;
*)
	echo "trim-quality-cache: repository root must be absolute: $repository_root" >&2
	exit 2
	;;
esac
case $budget_gib in
'' | *[!0-9]*)
	echo "trim-quality-cache: budget must be a whole GiB number: $budget_gib" >&2
	exit 2
	;;
esac

# Cleanup validates the exact owned path before removal: an existing
# non-symlink directory outside the repository, never a filesystem root.
if [ ! -d "$cache_root" ] || [ -L "$cache_root" ]; then
	echo "trim-quality-cache: cache root is not an existing non-symlink directory: $cache_root" >&2
	exit 2
fi
if [ "$cache_root" = / ] || [ "$cache_root" = "$repository_root" ]; then
	echo "trim-quality-cache: refusing to trim a filesystem or repository root: $cache_root" >&2
	exit 2
fi
case $cache_root/ in
"$repository_root"/*)
	echo "trim-quality-cache: cache root must stay outside the repository: $cache_root" >&2
	exit 2
	;;
esac

build_cache=$cache_root/go-build
if [ ! -d "$build_cache" ]; then
	echo "quality-cache-trim: no go-build cache under $cache_root; nothing to trim"
	exit 0
fi

size_kib() {
	du -sk "$1" 2>/dev/null | awk '{print $1}'
}

before_kib=$(size_kib "$build_cache")
budget_kib=$((budget_gib * 1024 * 1024))
if [ "${before_kib:-0}" -le "$budget_kib" ]; then
	echo "quality-cache-trim: go-build ${before_kib} KiB within ${budget_gib} GiB budget; retained"
	exit 0
fi

echo "quality-cache-trim: go-build ${before_kib} KiB exceeds ${budget_gib} GiB budget; trimming"
GOCACHE=$build_cache go clean -cache
after_kib=$(size_kib "$build_cache")
echo "quality-cache-trim: go-build ${before_kib} KiB -> ${after_kib:-0} KiB"