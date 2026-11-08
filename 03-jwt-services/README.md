# 03 · JWT-SVIDs between services

The same two services as [project 02](../02-mtls-services), but the client proves who it is with a
**JWT in an HTTP header** instead of a client certificate. This project shows how JWTs behave in practice,
including the main weakness: **a stolen token can be replayed.**

```
 client (uid 1002)                                         server (uid 1001)
   │  1. asks agent: "JWT for audience .../server"            │
   │  2. TLS handshake: only the SERVER shows a cert ────────►│
   │  3. GET /  Authorization: Bearer eyJhbGci... ───────────►│ 4. check signature + expiry + audience
   │                                                          │ 5. check sub == spiffe://example.org/client
   │◄──────────────────────────────── 200 hello ──────────────│
```

## Run it

Needs Docker (on a Mac: `brew install colima docker docker-compose && colima start`). From this folder:

```sh
./scripts/up.sh                  # about 1 minute the first time (builds the Go app)
docker compose logs -f client    # 200: hello spiffe://example.org/client ...
./scripts/down.sh                # stop and delete all state when you're done
```

## mTLS (02) vs JWT (03)

| | 02 mTLS | 03 JWT |
|---|---|---|
| Client proves identity with | Client certificate | `Authorization: Bearer <JWT>` |
| TLS | Both sides show certs | Only the server shows a cert |
| Intruder is rejected | During the handshake, before any HTTP | After the request arrives: **403** |
| Stolen credential | Useless without the private key | **Works for anyone until it expires** |
| Works through proxies that end TLS | ❌ | ✅ |

## Exercises

Run each from this folder with the stack up (`./scripts/up.sh`).

### A. The intruder gets a valid token, but the wrong one (2 min)

```sh
docker compose --profile intruder up -d intruder
docker compose logs -f intruder server
```
- intruder: `403: spiffe://example.org/intruder is not allowed`
- server: `403: valid token, but spiffe://example.org/intruder is not allowed`

The token is **genuine**: SPIRE signed it, and the signature checks out. It just belongs to the wrong identity.
Checking that a token is valid and checking who it belongs to are two separate steps (see `runServer` in `app/main.go`).

### B. Wrong audience (2 min)

Run a client that asks SPIRE for a token addressed to a *different* service:

```sh
docker compose run --rm --no-deps -e AUDIENCE=spiffe://example.org/some-other-api client
```
```
401: invalid token: jwtsvid: expected audience in ["spiffe://example.org/server"] (audience=["spiffe://example.org/some-other-api"])
```
Press `Ctrl+C` to stop it.

**Why it matters:** if service A sends its token to service B, B can't reuse it against A. The audience pins
each token to one receiver.

### C. Steal a token and replay it (5 min) ⭐ the key exercise

1. Copy the client's current token from its logs:
   ```sh
   TOKEN=$(docker compose logs --no-log-prefix client | grep '^TOKEN=' | tail -1 | cut -d= -f2)
   ```
2. Send it from a completely different container, with no SPIRE identity at all:
   ```sh
   docker run --rm --network 03-jwt-services_default curlimages/curl:8.11.1 \
     -sk -H "Authorization: Bearer $TOKEN" https://server:8443/
   ```
   ```
   hello spiffe://example.org/client, ...     ← the server thinks you're the client
   ```
3. Change one character and try again (add `x` to the end):
   ```sh
   docker run --rm --network 03-jwt-services_default curlimages/curl:8.11.1 \
     -sk -H "Authorization: Bearer ${TOKEN}x" https://server:8443/
   ```
   ```
   invalid token: ... error in cryptographic primitive     ← tampering breaks the signature
   ```
4. Wait **3 minutes**, then repeat step 2:
   ```
   invalid token: jwtsvid: token has expired
   ```

**Why 3 minutes and not 2?** Tokens live 2 minutes (`default_jwt_svid_ttl = "2m"` in `server.conf`),
but the JWT library accepts tokens up to **1 minute past expiry** to allow for clocks that are slightly off
(`DefaultLeeway` in go-jose). So a stolen token works for *lifetime + 1 minute*.

**What you learned:**
- JWTs are **bearer tokens**: whoever holds one *is* that identity, until it expires.
- They can't be edited, because of the signature, but they **can be copied**.
- Defenses: short lifetimes, a narrow audience, and always sending them over TLS.
- Compare project 02: a stolen X.509 cert is useless without its private key.

## How the code works

**Client** (`runClient`):
```go
svid, _ := jwtSource.FetchJWTSVID(ctx, jwtsvid.Params{Audience: "spiffe://example.org/server"})
req.Header.Set("Authorization", "Bearer "+svid.Marshal())
```

**Server** (`runServer`):
```go
svid, err := jwtsvid.ParseAndValidate(token, jwtSource, []string{audience}) // signature, expiry, audience
if svid.ID != allowed { /* 403 */ }                                         // who is it?
```

The server gets SPIRE's **public** signing keys from its own agent (`jwtSource`), so checking a token
needs no network call to the SPIRE server.

## Files

| File | What it is |
|---|---|
| `app/main.go` | Go client/server using JWT-SVIDs |
| `conf/server/server.conf` | Same as 02, plus `default_jwt_svid_ttl = "2m"` |
| `docker-compose.yml`, `scripts/` | Same as 02 |
