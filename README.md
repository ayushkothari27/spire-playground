# spire-playground

Hands-on projects for learning [SPIFFE](https://spiffe.io) and [SPIRE](https://github.com/spiffe/spire),
the open standard and reference implementation for giving workloads identities that rotate on their own.
They replace API keys and long-lived certificates.

Everything runs locally in Docker. Each project is self-contained.

| # | Project | What you learn |
|---|---|---|
| 01 | [**SPIRE quickstart**](01-spire-quickstart) | Run a server and agent, register a workload by hand, fetch X.509 and JWT SVIDs, and see attestation allow or deny |
| 02 | [**mTLS between services**](02-mtls-services) | Two Go services talking over mTLS with SPIFFE IDs, certs rotating every minute, and an intruder being rejected |
| 03 | [**JWT between services**](03-jwt-services) | The same services using JWT-SVIDs: audience checks, 403 vs handshake rejection, and replaying a stolen token |

## Prerequisites

- Docker. On a Mac: `brew install colima docker docker-compose && colima start`
- Go 1.27+ (optional; only needed to change the Go code outside Docker)

## Quick start

```sh
cd 01-spire-quickstart && ./scripts/up.sh      # then follow its README
cd 02-mtls-services   && ./scripts/up.sh && docker compose logs -f client
cd 03-jwt-services    && ./scripts/up.sh && docker compose logs -f client
```

Each project has a `scripts/down.sh` that removes all of its containers and state.

## Exercises

| Exercise | Where | Time |
|---|---|---|
| 1. Two identities on one machine | [01 README → Exercises](01-spire-quickstart#exercises) | 10 min |
| 3. Several selectors = all must match | [01 README → Exercises](01-spire-quickstart#exercises) | 10 min |
| 4. Freeze the agent, watch certs expire | [02 README → Exercise 4](02-mtls-services#exercise-4-freeze-the-agent-and-watch-certs-expire-5-min) | 5 min |
| A–C. Intruder, wrong audience, stolen token | [03 README → Exercises](03-jwt-services#exercises) | 10 min |

## Versions

SPIRE 1.14.0 · go-spiffe v2.9.0 · Go 1.27
