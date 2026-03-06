#!/usr/bin/env bash
set -euo pipefail

CHAIND="./chaind"

if [[ ! -x "$CHAIND" ]]; then
  echo "Building chaind..."
  go build -o chaind .
fi

# Assert that a jq expression evaluates to "true" against the given JSON.
assert() {
  local description="$1"
  local json="$2"
  local expr="$3"

  if ! echo "$json" | jq -e "$expr" > /dev/null; then
    echo "FAIL: $description"
    echo "      expression: $expr"
    echo "      output:     $json"
    exit 1
  fi

  echo "PASS: $description"
}

echo "Pulling test images..."
docker pull alpine:3.21 -q
docker pull alpine:3.20 -q

echo "Building test images..."

# Single-layer derived image (alpine layer + 1 RUN = 2 layers total)
docker build -t chaind-test:latest -f- . <<'EOF'
FROM alpine:3.21
RUN echo "extra layer" > /extra
EOF

# Multi-layer base image (alpine layer + 2 RUN = 3 layers total)
docker build -t chaind-base:latest -f- . <<'EOF'
FROM alpine:3.21
RUN echo "layer1" > /layer1
RUN echo "layer2" > /layer2
EOF

# Derived from multi-layer base (3 base layers + 1 RUN = 4 layers total)
docker build -t chaind-derived:latest -f- . <<'EOF'
FROM chaind-base:latest
RUN echo "layer3" > /layer3
EOF

echo

out=$($CHAIND compare alpine:3.21 chaind-test:latest)
assert "CONFIRMED_BASE: alpine:3.21 → chaind-test:latest" "$out" '.verdict == "CONFIRMED_BASE"'
assert "CONFIRMED_BASE: base is alpine:3.21" "$out" '.base == "alpine:3.21"'
assert "CONFIRMED_BASE: derived is chaind-test:latest" "$out" '.derived == "chaind-test:latest"'
assert "CONFIRMED_BASE: images contains both refs" "$out" '(.images | keys | length) == 2'
assert "CONFIRMED_BASE: 1 matched layer" "$out" '.matched_layers | length == 1'
assert "CONFIRMED_BASE: 1 extra layer" "$out" '.extra_layers | length == 1'

out=$($CHAIND compare chaind-test:latest alpine:3.21)
assert "CONFIRMED_BASE reversed: base is still alpine:3.21" "$out" '.base == "alpine:3.21"'
assert "CONFIRMED_BASE reversed: derived is chaind-test:latest" "$out" '.derived == "chaind-test:latest"'

out=$($CHAIND compare chaind-base:latest chaind-derived:latest)
assert "CONFIRMED_BASE: chaind-base:latest → chaind-derived:latest" "$out" '.verdict == "CONFIRMED_BASE"'
assert "CONFIRMED_BASE: base is chaind-base:latest" "$out" '.base == "chaind-base:latest"'

out=$($CHAIND compare alpine:3.20 chaind-test:latest || true)
assert "NOT_BASE: verdict" "$out" '.verdict == "NOT_BASE"'
assert "NOT_BASE: base is null" "$out" '.base == null'
assert "NOT_BASE: derived is null" "$out" '.derived == null'
assert "NOT_BASE: images contains both refs" "$out" '(.images | keys | length) == 2'
assert "NOT_BASE: matched_layers is empty" "$out" '.matched_layers == []'

out=$($CHAIND compare alpine:3.21 alpine:3.21 || true)
assert "SAME_IMAGE: verdict" "$out" '.verdict == "SAME_IMAGE"'
assert "SAME_IMAGE: base is null" "$out" '.base == null'
assert "SAME_IMAGE: derived is null" "$out" '.derived == null'

echo

graph=$($CHAIND graph)
# Expected chains among test images:
#   chain 1: alpine:3.21 → chaind-test:latest
#   chain 2: alpine:3.21 → chaind-base:latest → chaind-derived:latest
# alpine:3.20 must appear in unrelated (other daemon images may also be present)
assert "GRAPH: chain count is 2" "$graph" '.chains | length == 2'
assert "GRAPH: alpine:3.21 is a chain root" "$graph" '[.chains[].nodes[0].reference] | any(. == "alpine:3.21")'
assert "GRAPH: chaind-test:latest is a chain leaf" "$graph" '[.chains[].nodes[-1].reference] | any(. == "chaind-test:latest")'
assert "GRAPH: chaind-derived:latest is a chain leaf" "$graph" '[.chains[].nodes[-1].reference] | any(. == "chaind-derived:latest")'
assert "GRAPH: alpine:3.20 is unrelated" "$graph" '[.unrelated[].reference] | any(. == "alpine:3.20")'
assert "GRAPH: alpine:3.21 is not unrelated" "$graph" '[.unrelated[].reference] | any(. == "alpine:3.21") | not'
