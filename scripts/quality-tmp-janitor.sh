#!/bin/sh
# Remove disposable ardents verification workdirs from the system temporary
# directory once no live process uses them. A verification session calls this
# with --own for the exact paths it created when its run completes, and may
# sweep older orphans left by finished or crashed sessions. The shared quality
# cache root is never removed; `make quality-cache-trim` bounds its size.
set -eu

usage() {
	echo "usage: $0 [--dry-run] [--temp-root DIR] [--max-age-hours N] [--keep NAME]... [--own PATH]..." >&2
	exit 2
}

dry_run=0
own_mode=0
temp_root=/tmp
max_age_hours=24
keep_names="ardents-network-quality"
own_paths=""

while [ "$#" -gt 0 ]; do
	case $1 in
	--dry-run)
		dry_run=1
		shift
		;;
	--own)
		own_mode=1
		shift
		;;
	--temp-root)
		[ "$#" -ge 2 ] || usage
		temp_root=$2
		shift 2
		;;
	--max-age-hours)
		[ "$#" -ge 2 ] || usage
		max_age_hours=$2
		shift 2
		;;
	--keep)
		[ "$#" -ge 2 ] || usage
		keep_names="$keep_names
$2"
		shift 2
		;;
	/*)
		own_mode=1
		own_paths="$own_paths
$1"
		shift
		;;
	*)
		usage
		;;
	esac
done

case $max_age_hours in
'' | *[!0-9]*)
	echo "quality-tmp-janitor: --max-age-hours must be a whole number: $max_age_hours" >&2
	exit 2
	;;
esac

if [ ! -d "$temp_root" ]; then
	echo "quality-tmp-janitor: temp root does not exist: $temp_root" >&2
	exit 2
fi
temp_root=$(cd "$temp_root" && pwd -P)

is_kept() {
	printf '%s\n' "$keep_names" | grep -Fxq "$1"
}

# One lsof pass over all open files marks every temp child a live process
# still uses (open file, cwd or mapped image), so a parallel session's
# workdir is never removed. Without lsof, only explicitly owned paths may
# be removed.
in_use_names=""
if command -v lsof >/dev/null 2>&1; then
	in_use_names=$(lsof -Fn 2>/dev/null | awk -v root="$temp_root" '
		substr($0, 1, 1) == "n" {
			path = substr($0, 2)
			if (index(path, root "/") == 1) {
				rest = substr(path, length(root) + 2)
				slash = index(rest, "/")
				name = (slash > 0) ? substr(rest, 1, slash - 1) : rest
				if (name != "") print name
			}
		}' | sort -u) || in_use_names=""
elif [ "$own_mode" -eq 0 ]; then
	echo "quality-tmp-janitor: lsof is unavailable; sweep requires it to protect live workdirs" >&2
	exit 2
fi

is_in_use() {
	printf '%s\n' "$in_use_names" | grep -Fxq "$1"
}

remove_path() {
	path=$1
	name=${path##*/}
	if [ ! -e "$path" ] && [ ! -L "$path" ]; then
		echo "quality-tmp-janitor: skip (absent): $path"
		return 0
	fi
	if [ -L "$path" ]; then
		echo "quality-tmp-janitor: skip (symlink): $path"
		return 0
	fi
	# Normalize the parent so /tmp and /private/tmp forms compare equal.
	parent=${path%/*}
	if [ -d "$parent" ]; then
		path=$(cd "$parent" && pwd -P)/$name
	fi
	case $path in
	"$temp_root"/*) ;;
	*)
		echo "quality-tmp-janitor: skip (outside $temp_root): $path"
		return 0
		;;
	esac
	case $name in
	ardents-*) ;;
	*)
		echo "quality-tmp-janitor: skip (not ardents-*): $path"
		return 0
		;;
	esac
	if is_kept "$name"; then
		echo "quality-tmp-janitor: skip (kept shared root): $path"
		return 0
	fi
	if is_in_use "$name"; then
		echo "quality-tmp-janitor: skip (in use by a live process): $path"
		return 0
	fi
	if [ "$dry_run" -eq 1 ]; then
		echo "quality-tmp-janitor: would remove: $path"
		return 0
	fi
	rm -rf -- "$path"
	echo "quality-tmp-janitor: removed: $path"
}

if [ "$own_mode" -eq 1 ]; then
	printf '%s\n' "$own_paths" | while IFS= read -r path; do
		[ -n "$path" ] || continue
		remove_path "$path"
	done
	exit 0
fi

cutoff_minutes=$((max_age_hours * 60))
find "$temp_root" -maxdepth 1 -name 'ardents-*' -mmin +"$cutoff_minutes" -print |
	while IFS= read -r path; do
		name=${path##*/}
		if [ -L "$path" ]; then
			echo "quality-tmp-janitor: skip (symlink): $path"
			continue
		fi
		if [ ! -O "$path" ]; then
			echo "quality-tmp-janitor: skip (not owned by current user): $path"
			continue
		fi
		remove_path "$path"
	done