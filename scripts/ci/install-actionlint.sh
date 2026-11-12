#!/usr/bin/env bash
# Install a pinned actionlint release (linux_amd64) without sudo.
#
# Used by .github/workflows/scope-check.yml so workflow YAML is validated on
# every PR. Prints nothing but actionlint's version; the caller puts the
# destination directory on PATH.
set -euo pipefail

version="1.7.7"
sha256="023070a287cd8cccd71515fedc843f1985bf96c436b7effaecce67290e7e0757"
dest="${1:-${RUNNER_TEMP:-/tmp}/actionlint-bin}"

case "$(uname -s)/$(uname -m)" in
    Linux/x86_64) ;;
    *)
        echo "install-actionlint: unsupported platform $(uname -s)/$(uname -m)" >&2
        exit 1
        ;;
esac

archive="actionlint_${version}_linux_amd64.tar.gz"
url="https://github.com/rhysd/actionlint/releases/download/v${version}/${archive}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$url" -o "$tmp/$archive"
echo "${sha256}  $tmp/${archive}" | sha256sum -c -
tar -xzf "$tmp/$archive" -C "$tmp" actionlint
mkdir -p "$dest"
install -m 0755 "$tmp/actionlint" "$dest/actionlint"

"$dest/actionlint" --version
