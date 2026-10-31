#!/usr/bin/env bash
# Stops everything and deletes all SPIRE state, so the next up.sh starts fresh.
set -euo pipefail
cd "$(dirname "$0")/.."

# Stop all containers (including the intruder) and delete their volumes (-v).
docker compose --profile intruder down -v

# The bundle and join token are tied to the old server, so delete them too.
rm -f conf/agent/bootstrap.crt .env
