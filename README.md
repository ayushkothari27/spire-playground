# spire-playground

Hands-on projects for learning [SPIFFE](https://spiffe.io) and [SPIRE](https://github.com/spiffe/spire),
the open standard and reference implementation for giving workloads identities that rotate on their own.
They replace API keys and long-lived certificates.

Everything runs locally in Docker. Each project is self-contained.

| # | Project | What you learn |
|---|---|---|
| 01 | [**SPIRE quickstart**](01-spire-quickstart) | Run a server and agent, register a workload by hand, fetch X.509 and JWT SVIDs, and see attestation allow or deny |
| 02 | [**mTLS between services**](02-mtls-services) | Two Go services talking over mTLS with SPIFFE IDs, certs rotating every minute, and an intruder being rejected |

## Prerequisites

- Docker. On a Mac: `brew install colima docker docker-compose && colima start`
- Go 1.27+ (optional; only needed to change project 02's code outside Docker)

## Quick start

```sh
cd 01-spire-quickstart && ./scripts/up.sh      # then follow its README
cd 02-mtls-services   && ./scripts/up.sh && docker compose logs -f client
```

Each project has a `scripts/down.sh` that removes all of its containers and state.

## Versions

SPIRE 1.14.0 · go-spiffe v2.9.0 · Go 1.27
