#!/usr/bin/env bash
# Starts everything in order:
#   1. SPIRE server
#   2. Give the agent the server's CA bundle and a one-time join token
#   3. SPIRE agent
#   4. Register workload identities
#   5. Demo services

# Stop on any error, unset variable, or failed pipe.
set -euo pipefail
# Run from the repo root no matter where the script is called from.
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
  # -spiffeID gives the agent a friendly alias (spiffe://example.org/agent) to use as a parent below.
  echo "JOIN_TOKEN=$(server token generate -spiffeID spiffe://example.org/agent | awk '{print $2}')" > .env
fi

echo "==> Starting SPIRE agent"
# Compose reads JOIN_TOKEN from .env automatically.
docker compose up -d --wait spire-agent

echo "==> Registering workloads"
# Each entry means: "a process with UID X, on an agent with ID parentID, gets SPIFFE ID Y".
register() {
  # Skip entries that already exist, so the script is safe to re-run.
  if server entry show -spiffeID "$1" | grep -q "Entry ID"; then
    echo "   $1 already registered"
  else
    # -x509SVIDTTL 60 = certs expire after 60s, so you can watch rotation.
    server entry create -parentID spiffe://example.org/agent \
      -spiffeID "$1" -selector "unix:uid:$2" -x509SVIDTTL 60 >/dev/null
    echo "   $1 <- unix:uid:$2"
  fi
}
register spiffe://example.org/server   1001
register spiffe://example.org/client   1002
register spiffe://example.org/intruder 1003

echo "==> Starting demo services"
# --build compiles the Go app the first time (and again after code changes).
docker compose up -d --build server client

echo
echo "Done. Watch the client:   docker compose logs -f client"
echo "Try the intruder:         docker compose --profile intruder up -d intruder"
