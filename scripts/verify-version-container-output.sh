#!/usr/bin/env bash

set -euo pipefail

image_ref="${1:?image ref is required}"
expected_version="${2:?expected version is required}"
expected_commit="${3:?expected commit is required}"
expected_build_date="${4:-}"

runner="$(mktemp)"
cleanup() {
	rm -f "$runner"
}
trap cleanup EXIT

cat > "$runner" <<EOF
#!/usr/bin/env bash
docker run --rm "${image_ref}" "\$@"
EOF

chmod +x "$runner"

"$(dirname -- "$0")/verify-version-output.sh" "$runner" "$expected_version" "$expected_commit" "$expected_build_date"
