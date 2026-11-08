// JWT-SVID demo. Run with MODE=server or MODE=client.
// Unlike project 02, the client proves who it is with a JWT in an HTTP header,
// not with a client certificate. The connection is normal TLS: only the server shows a cert.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
	ctx := context.Background()
	socket := workloadapi.WithClientOptions(workloadapi.WithAddr(
		env("SPIFFE_ENDPOINT_SOCKET", "unix:///run/spire/sockets/agent.sock")))

	// X.509 source: the server's TLS cert, and the CA bundle the client uses to check it.
	x509Source, err := workloadapi.NewX509Source(ctx, socket)
	if err != nil {
		log.Fatalf("could not get an X.509 identity from the SPIRE agent: %v", err)
	}
	defer x509Source.Close()

	// JWT source: lets the client request JWT-SVIDs, and gives the server the public keys to check them.
	jwtSource, err := workloadapi.NewJWTSource(ctx, socket)
	if err != nil {
		log.Fatalf("could not connect to the JWT side of the SPIRE agent: %v", err)
	}
	defer jwtSource.Close()

	svid, _ := x509Source.GetX509SVID()
	log.Printf("I am %s", svid.ID)

	switch env("MODE", "") {
	case "server":
		runServer(x509Source, jwtSource)
	case "client":
		runClient(ctx, x509Source, jwtSource)
	default:
		log.Fatal("set MODE=server or MODE=client")
	}
}

func runServer(x509Source *workloadapi.X509Source, jwtSource *workloadapi.JWTSource) {
	// Tokens must be addressed to us (the "aud" claim) and come from this SPIFFE ID (the "sub" claim).
	audience := env("AUDIENCE", "spiffe://example.org/server")
	allowed := spiffeid.RequireFromString(env("ALLOWED_CLIENT_ID", "spiffe://example.org/client"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Get the token from "Authorization: Bearer <token>".
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			log.Printf("401: no token")
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		// 2. Check the signature (with SPIRE's public keys), the expiry, and the audience.
		svid, err := jwtsvid.ParseAndValidate(token, jwtSource, []string{audience})
		if err != nil {
			log.Printf("401: invalid token: %v", err)
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// 3. The token is genuine. Now check WHO it belongs to.
		if svid.ID != allowed {
			log.Printf("403: valid token, but %s is not allowed", svid.ID)
			http.Error(w, fmt.Sprintf("%s is not allowed", svid.ID), http.StatusForbidden)
			return
		}

		log.Printf("200: request from %s (token expires %s)", svid.ID, svid.Expiry.Format(time.TimeOnly))
		fmt.Fprintf(w, "hello %s, the time is %s", svid.ID, time.Now().Format(time.TimeOnly))
	})

	srv := &http.Server{
		Addr: ":8443",
		// Plain TLS: we show our cert, but we DON'T ask the client for one.
		TLSConfig: tlsconfig.TLSServerConfig(x509Source),
	}
	log.Printf("listening on :8443, accepting JWTs for audience %s from %s", audience, allowed)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

func runClient(ctx context.Context, x509Source *workloadapi.X509Source, jwtSource *workloadapi.JWTSource) {
	serverID := spiffeid.RequireFromString(env("SERVER_ID", "spiffe://example.org/server"))
	url := env("SERVER_URL", "https://server:8443/")
	// Who the token is for. The server rejects tokens addressed to anyone else.
	audience := env("AUDIENCE", "spiffe://example.org/server")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// We still check the server's SPIFFE ID, but we present no client cert.
			TLSClientConfig: tlsconfig.TLSClientConfig(x509Source, tlsconfig.AuthorizeID(serverID)),
		},
	}

	lastToken := ""
	for {
		// Ask the agent for a JWT addressed to the server. The agent caches it and
		// returns a fresh one shortly before it expires.
		svid, err := jwtSource.FetchJWTSVID(ctx, jwtsvid.Params{Audience: audience})
		if err != nil {
			log.Printf("could not get a JWT: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		token := svid.Marshal()

		// Print the full token whenever it changes (used in the "steal a token" exercise).
		if token != lastToken {
			log.Printf("new token, expires %s:\nTOKEN=%s", svid.Expiry.Format(time.TimeOnly), token)
			lastToken = token
		}

		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("FAILED: %v", err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("%d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		time.Sleep(5 * time.Second)
	}
}

// env reads an environment variable and falls back to def when it isn't set.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
