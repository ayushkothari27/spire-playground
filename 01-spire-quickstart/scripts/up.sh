#!/usr/bin/env bash
# Boots a SPIRE server and agent. Registering and fetching identities is up to you (see README).

# Stop on any error, unset variable, or failed pipe.
set -euo pipefail
# Run from the project folder no matter where the script is called from.
cd "$(dirname "$0")/.."

# Shortcut: run a spire-server CLI command inside the server container.
server() { docker compose exec -T spire-server /opt/spire/bin/spire-server "$@"; }

echo "==> Starting SPIRE server"
# --wait blocks until the healthcheck passes.
docker compose up -d --wait spire-server

echo "==> Writing trust bundle for the agent"
# The server's CA cert. The agent uses it to check it's talking to the real server.
server bundle show > conf/agent/bootstrap.crt

# Tokens are single-use, so only make one if we don't already have one.
if [ ! -f .env ]; then
  echo "==> Creating a one-time join token for the agent (saved in .env)"
  # -spiffeID gives the agent a friendly alias you can use as -parentID when registering.
  echo "JOIN_TOKEN=$(server token generate -spiffeID spiffe://example.org/agent | awk '{print $2}')" > .env
fi

echo "==> Starting SPIRE agent"
# Folder for fetched certs; world-writable so any UID inside the container can write to it.
mkdir -p out && chmod 777 out
# Compose reads JOIN_TOKEN from .env automatically.
docker compose up -d --wait spire-agent

echo
echo "SPIRE is up. Follow the steps in README.md, starting at step 2."
