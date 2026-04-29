#!/usr/bin/env bash
# compose/teardown.sh
set -euo pipefail
cd "$(dirname "$0")"
docker compose -f docker-compose.yml down -v
echo "==> Teardown complete."
