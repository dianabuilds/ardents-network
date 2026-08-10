#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  printf 'usage: %s LOCK OUTPUT_DIRECTORY\n' "$0" >&2
  exit 64
fi

lock=$(realpath "$1")
output=$2
mkdir -p "$output/packages"
mkdir -p "$output/wheelhouse"
output=$(realpath "$output")
[[ "$output" != / && "$output" != "$PWD" ]] || exit 64

download_package() {
  local name=$1 version=$2 filename=$3 digest=$4 url=$5 expected_architecture=amd64
  [[ "$filename" == *_all.deb ]] && expected_architecture=all
  curl --fail --location --proto '=https' --tlsv1.2 --output "$output/packages/$filename" "$url"
  printf '%s  %s\n' "$digest" "$output/packages/$filename" | sha256sum --check --status
  [[ $(dpkg-deb --field "$output/packages/$filename" Package) == "$name" ]]
  [[ $(dpkg-deb --field "$output/packages/$filename" Version) == "$version" ]]
  [[ $(dpkg-deb --field "$output/packages/$filename" Architecture) == "$expected_architecture" ]]
}

while IFS=$'\t' read -r kind name version filename digest url license; do
  case "$kind/$name" in
    package/*) download_package "$name" "$version" "$filename" "$digest" "$url" ;;
    tor/package) download_package tor "$version" "$filename" "$digest" "$url" ;;
  esac
done < "$lock"

while IFS=$'\t' read -r kind name version filename digest url license; do
  [[ "$kind" == wheel ]] || continue
  curl --fail --location --proto '=https' --tlsv1.2 --output "$output/wheelhouse/$filename" "$url"
  printf '%s  %s\n' "$digest" "$output/wheelhouse/$filename" | sha256sum --check --status
done < "$lock"

revision=$(awk -F '\t' '$1 == "chutney" && $2 == "revision" {print $3}' "$lock")
archive=$(awk -F '\t' '$1 == "chutney" && $2 == "revision" {print $4}' "$lock")
archive_digest=$(awk -F '\t' '$1 == "chutney" && $2 == "revision" {print $5}' "$lock")
repository=$(awk -F '\t' '$1 == "chutney" && $2 == "revision" {print $6}' "$lock")
[[ ${#revision} -eq 40 && -n "$archive" && ${#archive_digest} -eq 64 && -n "$repository" ]]

checkout="$output/chutney-checkout"
git clone --filter=blob:none --no-checkout "$repository" "$checkout"
git -C "$checkout" checkout --detach "$revision"
[[ $(git -C "$checkout" rev-parse HEAD) == "$revision" ]]
# A plain git-generated tar is byte-for-byte reproducible. The tar.gz writer
# embeds the current gzip timestamp, so its digest changes on every run.
git -C "$checkout" archive --format=tar --prefix=chutney/ -o "$output/$archive" "$revision"
printf '%s  %s\n' "$archive_digest" "$output/$archive" | sha256sum --check --status
rm -rf "$checkout"

cp "$lock" "$output/reference.lock"
printf 'prepared pinned raw Tor packages, wheels, and Chutney archive in %s\n' "$output"
