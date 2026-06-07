#!/usr/bin/env bash

set -euo pipefail

binary_path="${1:?binary path is required}"
expected_version="${2:?expected version is required}"
expected_commit="${3:?expected commit is required}"
expected_build_date="${4:-}"

if [[ ! -x "$binary_path" ]]; then
	echo "binary is not executable: $binary_path" >&2
	exit 1
fi

output="$("$binary_path" version)"

require_fragment() {
	local fragment="$1"

	if [[ "$output" != *"$fragment"* ]]; then
		echo "missing fragment in version output: $fragment" >&2
		echo "$output" >&2
		exit 1
	fi
}

require_fragment "rancher-mcp-server"
require_fragment "Version:    ${expected_version}"
require_fragment "Git commit: ${expected_commit}"

if [[ -n "$expected_build_date" ]]; then
	require_fragment "Built:      ${expected_build_date}"
elif [[ "$output" == *"Built:      unknown"* ]]; then
	echo "build date was not injected" >&2
	echo "$output" >&2
	exit 1
fi

if [[ "$output" == *"Git commit: unknown"* ]]; then
	echo "git commit was not injected" >&2
	echo "$output" >&2
	exit 1
fi

echo "version output is aligned for $binary_path"
