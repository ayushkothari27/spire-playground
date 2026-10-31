# 02 · mTLS between services with SPIFFE

Two Go services talk over **mutual TLS** using certificates from SPIRE.
There are no cert files, no secrets in code, and certs rotate every minute without a restart.
A third "intruder" service runs the same code with a different identity, and the server rejects it.

```
 ┌──────────────┐  join token   ┌──────────────┐   Workload API (unix socket)
 │ spire-server │◄──────────────│ spire-agent  │◄───────────┬──────────────┬───────────────┐
 │  (the CA)    │   SVIDs       │              │            │              │               │
 └──────────────┘──────────────►└──────────────┘     ┌──────┴─────┐ ┌──────┴─────┐ ┌───────┴─────┐
                                                     │ server     │ │ client     │ │ intruder    │
                                                     │ uid 1001   │◄┤ uid 1002   │ │ uid 1003    │
                                                     └────────────┘ └────────────┘ └─────────────┘
                                                        ▲  mTLS ✔ (IDs checked)          │
                                                        └──────── ✘ rejected ────────────┘
```

New to SPIRE? Start with [`../01-spire-quickstart`](../01-spire-quickstart).

## Run it

Needs Docker (on a Mac: `brew install colima docker docker-compose && colima start`). From this folder:

```sh
./scripts/up.sh                  # start SPIRE, register 3 workloads, build and start the services
docker compose logs -f client    # watch it work
./scripts/down.sh                # stop and delete all state
```

Client output:

```
OK: "hello spiffe://example.org/client, the time is 21:29:29" | my cert serial …a94649 expires 21:30:06
OK: "hello spiffe://example.org/client, the time is 21:29:34" | my cert serial …735f8a expires 21:30:32   ← rotated
```

## Things to try

| Try | What you'll see |
|---|---|
| `docker compose --profile intruder up -d intruder`<br>`docker compose logs intruder server` | Intruder: `tls: bad certificate`. Server: `unexpected ID "spiffe://example.org/intruder"` |
| `docker compose stop spire-agent`, wait ~1 min, then `docker compose start spire-agent` | Certs expire, requests fail, then recover once the agent is back. Short TTLs limit the damage from a stolen cert. |
| Set `ALLOWED_CLIENT_ID: spiffe://example.org/intruder` on `server` in `docker-compose.yml`, then `docker compose up -d server` | The roles swap: the client is now rejected and the intruder gets in |
| `docker compose exec spire-server /opt/spire/bin/spire-server entry show` | The 3 registration entries (UID → SPIFFE ID) |

## How it works

1. **Node attestation.** The agent proves itself to the server with a one-time join token.
2. **Registration.** `up.sh` creates entries mapping `unix:uid:1001 → /server`, `1002 → /client`, and `1003 → /intruder`.
3. **Workload attestation.** The workload containers share the agent's process namespace
   (`pid: service:spire-agent`), so when one calls the socket, the agent can read the caller's UID.
4. **mTLS.** [`go-spiffe`](https://github.com/spiffe/go-spiffe) builds the TLS config from the SVID.
   Each side checks the **other side's SPIFFE ID** (`tlsconfig.AuthorizeID`), not a hostname.
5. **Rotation.** SVIDs live 60s. The agent renews them at about half their lifetime and pushes them to the app,
   which uses the new cert on the next handshake.

The key code in `app/main.go`:

```go
source, _ := workloadapi.NewX509Source(ctx)                              // live, auto-rotating cert
tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeID(clientID)) // server side
tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID)) // client side
```

## Files

| File | What it is |
|---|---|
| `app/main.go` | Go client/server (`MODE=server` or `MODE=client`) |
| `app/Dockerfile` | Two-stage build into a distroless image |
| `conf/server/server.conf` | SPIRE server config (1-minute SVIDs) |
| `conf/agent/agent.conf` | SPIRE agent config (`unix` workload attestor) |
| `docker-compose.yml` | SPIRE server and agent, plus server, client, and intruder |
| `scripts/up.sh` / `down.sh` | Bootstrap, register, and teardown |

## Ideas to extend

- Use JWT-SVIDs instead of mTLS for one hop.
- Swap the `unix` attestor for the `docker` attestor (selectors such as `docker:label:app:client`).
- Put Envoy in front and have it fetch certs from the agent's SDS API, so the app has no TLS code at all.
