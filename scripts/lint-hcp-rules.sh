#!/bin/bash
# Lint HCP tenant rules using promtool
# Extracts PrometheusRule specs from the OpenShift Template and validates them
#
# Usage: ./lint-hcp-rules.sh [promtool-path] [yq-path]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HCP_YAML="${SCRIPT_DIR}/../resources/tenant-rules/hcp.yaml"
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Use provided paths or fall back to system binaries
PROMTOOL="${1:-promtool}"
YQ="${2:-yq}"

if ! command -v "$PROMTOOL" &> /dev/null && [[ ! -x "$PROMTOOL" ]]; then
    echo "Error: promtool not found at $PROMTOOL" >&2
    exit 1
fi

if ! command -v "$YQ" &> /dev/null && [[ ! -x "$YQ" ]]; then
    echo "Error: yq not found at $YQ" >&2
    exit 1
fi

echo "Linting HCP rules from ${HCP_YAML}"

# Extract each PrometheusRule spec from the template
# The template has .objects[] array with PrometheusRule resources
RULE_COUNT=$("$YQ" eval '.objects | length' "$HCP_YAML")

ERRORS=0
for i in $(seq 0 $((RULE_COUNT - 1))); do
    NAME=$("$YQ" eval ".objects[$i].metadata.name" "$HCP_YAML")
    KIND=$("$YQ" eval ".objects[$i].kind" "$HCP_YAML")

    # Only process PrometheusRule objects
    if [[ "$KIND" != "PrometheusRule" ]]; then
        continue
    fi

    RULE_FILE="$TMP_DIR/${NAME}.yaml"

    # Extract the spec.groups and wrap in a format promtool understands
    "$YQ" eval ".objects[$i].spec" "$HCP_YAML" > "$RULE_FILE"

    echo -n "  Checking ${NAME}... "
    if "$PROMTOOL" check rules "$RULE_FILE" 2>&1; then
        echo "OK"
    else
        echo "FAILED"
        ERRORS=$((ERRORS + 1))
    fi
done

if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "ERROR: $ERRORS PrometheusRule(s) failed validation"
    exit 1
fi

echo ""
echo "All HCP rules passed validation"
