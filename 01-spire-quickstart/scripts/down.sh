#!/usr/bin/env bash
# Stops everything and deletes all SPIRE state, so the next up.sh starts fresh.
set -euo pipefail
cd "$(dirname "$0")/.."

# Stop the containers and delete their volumes (-v).
docker compose down -v

# The bundle, join token, and fetched certs belong to the old server, so delete them too.
rm -rf conf/agent/bootstrap.crt .env out
