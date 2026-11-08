# 01 · SPIRE quickstart

Run a SPIRE server and agent locally, then work through the core ideas by hand:
**attestation → registration → SVIDs**. It takes about 15 minutes.

```
 ┌──────────────┐  1. agent proves itself     ┌──────────────┐
 │ spire-server │◄────── (join token) ────────│ spire-agent  │◄── 3. a process asks:
 │  (the CA)    │                             │              │     "who am I?"
 │              │── 2. entries: uid → ID ────►│              │──► 4. agent checks its UID,
 └──────────────┘                             └──────────────┘     returns an SVID
```

## Concepts in one minute

| Term | Meaning |
|---|---|
| **SPIFFE ID** | A name for a workload, like `spiffe://example.org/myservice` |
| **Trust domain** | The `example.org` part. One SPIRE server is its root of trust. |
| **SVID** | A document that proves a SPIFFE ID: an X.509 cert (ID in the URI SAN) or a JWT |
| **Node attestation** | How an agent proves which machine it's on (here: a one-time join token) |
| **Workload attestation** | How the agent identifies a calling process (here: its Unix UID) |
| **Registration entry** | A rule like "a process with selector `unix:uid:1000` gets ID X" |
| **Workload API** | The Unix socket where processes ask the agent for their SVIDs. No secrets are needed to call it. |

## Prerequisites

Docker. On a Mac: `brew install colima docker docker-compose && colima start`.

## Walkthrough

Run every command from this folder.

### 1. Start SPIRE

```sh
./scripts/up.sh
```

This starts the server, gives the agent the server's CA bundle and a one-time join token, then starts the agent.

A shortcut for the server CLI, used below:

```sh
alias server='docker compose exec spire-server /opt/spire/bin/spire-server'
```

### 2. Check the agent attested

```sh
server agent list
```

You'll see an agent with an ID like `spiffe://example.org/spire/agent/join_token/<uuid>`. That ID came from **node attestation**.

### 3. Ask for an identity *before* registering

```sh
docker compose exec -u 1000 spire-agent \
  /opt/spire/bin/spire-agent api fetch x509 -socketPath /run/spire/sockets/agent.sock
```

```
rpc error: code = PermissionDenied desc = no identity issued
```

The agent sees a process with UID 1000, but no entry matches it, so it gets nothing.

### 4. Register a workload

```sh
server entry create \
  -parentID spiffe://example.org/agent \
  -spiffeID spiffe://example.org/myservice \
  -selector unix:uid:1000
```

- `-parentID`: which agent(s) may issue this ID. `spiffe://example.org/agent` is the alias `up.sh` gave our agent.
- `-selector`: what the process must look like. Here it means "running as UID 1000".

### 5. Fetch again

Wait about 5 seconds for the agent to sync, then:

```sh
docker compose exec -u 1000 spire-agent \
  /opt/spire/bin/spire-agent api fetch x509 -socketPath /run/spire/sockets/agent.sock -write /out
```

```
SPIFFE ID:        spiffe://example.org/myservice
SVID Valid Until: ... (1 hour from now)
Writing SVID #0 to file /out/svid.0.pem.
```

The cert, key, and CA bundle are now in `./out` on your Mac. Look inside:

```sh
openssl x509 -in out/svid.0.pem -noout -text | grep -E 'URI|Not After'
#   Not After : ...
#   URI:spiffe://example.org/myservice      ← the identity lives in the SAN
```

### 6. Prove the selector matters

```sh
docker compose exec -u 1001 spire-agent \
  /opt/spire/bin/spire-agent api fetch x509 -socketPath /run/spire/sockets/agent.sock
# → no identity issued
```

Same machine and same command, but a different UID, so no identity.

### 7. Bonus: get a JWT-SVID

```sh
docker compose exec -u 1000 spire-agent \
  /opt/spire/bin/spire-agent api fetch jwt -audience my-api -socketPath /run/spire/sockets/agent.sock
```

Paste the token into [jwt.io](https://jwt.io). `sub` is your SPIFFE ID and `aud` is `my-api`.
JWT-SVIDs are for cases where mTLS isn't possible, for example behind an L7 load balancer.

### 8. Explore and clean up

```sh
server entry show                                        # list entries
server entry delete -entryID <id from entry show>        # remove one (fetch fails again)
server bundle show                                       # the trust bundle (root CA)
docker compose logs spire-agent                          # watch attestation in the logs
./scripts/down.sh                                        # stop and delete everything
```

## Exercises

Start each exercise fresh (about 30 seconds):

```sh
./scripts/down.sh && ./scripts/up.sh
alias server='docker compose exec spire-server /opt/spire/bin/spire-server'
fetchas() { docker compose exec -u "$1" spire-agent /opt/spire/bin/spire-agent api fetch x509 -socketPath /run/spire/sockets/agent.sock; }
```

`fetchas 1000` means "run `fetch` as UID 1000". `fetchas 1002:2000` means UID 1002, group 2000.

After creating an entry, **wait about 10 seconds** before fetching. The agent picks up new entries from the server every 5 seconds or so.
If you get `no identity issued` right after creating one, wait 5 more seconds and retry.

### Exercise 1: two identities on one machine (10 min)

**Goal:** see that *which process is calling* decides the identity.

1. Register two workloads:
   ```sh
   server entry create -parentID spiffe://example.org/agent -spiffeID spiffe://example.org/frontend -selector unix:uid:1000
   server entry create -parentID spiffe://example.org/agent -spiffeID spiffe://example.org/backend  -selector unix:uid:1001
   ```
2. Wait 10 seconds, then fetch as each user:
   ```sh
   fetchas 1000 | grep "SPIFFE ID"    # → spiffe://example.org/frontend
   fetchas 1001 | grep "SPIFFE ID"    # → spiffe://example.org/backend
   fetchas 1002                       # → no identity issued
   ```
3. Bonus: register a **second** entry for UID 1000 (`-spiffeID spiffe://example.org/frontend-v2 -selector unix:uid:1000`), then `fetchas 1000` again.
   You'll get `Received 2 svids`. One process can hold several identities.

**What you learned:** same machine and same command, but the caller's UID decides the identity.

### Exercise 3: several selectors = ALL must match (10 min)

**Goal:** see that selectors on one entry combine with AND.

1. Register an entry that needs UID 1002 **and** group 2000:
   ```sh
   server entry create -parentID spiffe://example.org/agent -spiffeID spiffe://example.org/admin \
     -selector unix:uid:1002 -selector unix:gid:2000
   ```
2. Wait 10 seconds, then try 3 callers:

   | Command | UID | Group | Result |
   |---|---|---|---|
   | `fetchas 1002:1002` | ✅ | ❌ | `no identity issued` |
   | `fetchas 1003:2000` | ❌ | ✅ | `no identity issued` |
   | `fetchas 1002:2000` | ✅ | ✅ | `spiffe://example.org/admin` |

**What you learned:** more selectors make an entry *stricter*. In real setups this is how you say
"only this user, running this binary, in this group."

## Files

| File | What it is |
|---|---|
| `conf/server/server.conf` | Server config: trust domain, TTLs, plugins (SQLite, join token, disk keys) |
| `conf/agent/agent.conf` | Agent config: server address, socket path, `unix` workload attestor |
| `docker-compose.yml` | The two containers |
| `scripts/up.sh` / `down.sh` | Bootstrap and teardown |

## Next

Go to [`../02-mtls-services`](../02-mtls-services), where two Go services use these identities for mTLS.
