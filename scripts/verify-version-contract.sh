#!/usr/bin/env bash

set -euo pipefail

repo_root="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
module_path="$(awk '/^module / {print $2}' "$repo_root/go.mod")"
version_pkg="${module_path}/pkg/core/version"

check_contains() {
	local file="$1"
	local pattern="$2"

	if ! grep -Fq -- "$pattern" "$file"; then
		echo "missing expected version target in ${file}: ${pattern}" >&2
		exit 1
	fi
}

check_absent() {
	local file="$1"
	local pattern="$2"

	if grep -Fq -- "$pattern" "$file"; then
		echo "found deprecated version target in ${file}: ${pattern}" >&2
		exit 1
	fi
}

check_contains "$repo_root/pkg/core/version/version.go" "GitCommit = "
check_contains "$repo_root/pkg/core/version/version.go" "BuildDate = "
check_contains "$repo_root/pkg/core/version/version.go" "const BinaryName = "

check_contains "$repo_root/Makefile" "PACKAGE = \$(shell go list -m)"
check_contains "$repo_root/Makefile" "VERSION_PKG = \$(PACKAGE)/pkg/core/version"
check_contains "$repo_root/Makefile" "-X '\$(VERSION_PKG).GitCommit="
check_contains "$repo_root/Makefile" "-X '\$(VERSION_PKG).Version="
check_contains "$repo_root/Makefile" "-X '\$(VERSION_PKG).BuildDate="

check_contains "$repo_root/.goreleaser.yml" "-X \"${version_pkg}.GitCommit="
check_contains "$repo_root/.goreleaser.yml" "-X \"${version_pkg}.Version="
check_contains "$repo_root/.goreleaser.yml" "-X \"${version_pkg}.BuildDate="

check_absent "$repo_root/Makefile" "/pkg/version."
check_absent "$repo_root/Makefile" ".CommitHash="
check_absent "$repo_root/Makefile" ".BuildTime="
check_absent "$repo_root/Makefile" ".BinaryName="

check_absent "$repo_root/.goreleaser.yml" "/pkg/version."
check_absent "$repo_root/.goreleaser.yml" ".CommitHash="
check_absent "$repo_root/.goreleaser.yml" ".BuildTime="
check_absent "$repo_root/.goreleaser.yml" ".BinaryName="

echo "version metadata contract is aligned"
