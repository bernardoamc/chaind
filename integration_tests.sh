#!/usr/bin/env bash
set -euo pipefail

CHAIND="./chaind"

if [[ ! -x "$CHAIND" ]]; then
  echo "Building chaind..."
  go build -o chaind .
fi

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
echo "=== CONFIRMED_BASE (1 matched layer): alpine:3.21 → chaind-test:latest ==="
$CHAIND alpine:3.21 chaind-test:latest
echo "Exit code: $?"

echo
echo "=== CONFIRMED_BASE (3 matched layers): chaind-base:latest → chaind-derived:latest ==="
$CHAIND chaind-base:latest chaind-derived:latest
echo "Exit code: $?"

echo
echo "=== NOT_BASE: alpine:3.20 → chaind-test:latest ==="
$CHAIND alpine:3.20 chaind-test:latest || echo "Exit code: $?"

echo
echo "=== SAME_IMAGE: alpine:3.21 → alpine:3.21 ==="
$CHAIND alpine:3.21 alpine:3.21 || echo "Exit code: $?"

echo
echo "=== JSON output ==="
$CHAIND alpine:3.21 chaind-test:latest --json | python3 -m json.tool --no-ensure-ascii 2>/dev/null || \
  $CHAIND alpine:3.21 chaind-test:latest --json

echo
echo "=== Quiet mode ==="
$CHAIND alpine:3.21 chaind-test:latest --quiet
