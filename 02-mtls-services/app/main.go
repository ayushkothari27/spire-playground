// A tiny mTLS demo. Run with MODE=server or MODE=client.
// Neither side has a certificate file: both get short-lived certs (X.509 SVIDs)
// from the local SPIRE agent, which replaces them automatically before they expire.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
	ctx := context.Background()

	// Connect to the SPIRE agent's Workload API socket.
	// X509Source keeps a live, auto-rotating copy of our cert and the trusted CA bundle.
	socket := env("SPIFFE_ENDPOINT_SOCKET", "unix:///run/spire/sockets/agent.sock")
	source, err := workloadapi.NewX509Source(ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(socket)))
	if err != nil {
		log.Fatalf("could not get an identity from the SPIRE agent: %v", err)
	}
	defer source.Close()

	// Print who SPIRE says we are. We never told it; it worked it out from our UID.
	svid, _ := source.GetX509SVID()
	log.Printf("I am %s (cert expires %s)", svid.ID, svid.Certificates[0].NotAfter.Format(time.TimeOnly))

	// Same binary, two roles.
	switch env("MODE", "") {
	case "server":
		runServer(source)
	case "client":
		runClient(source)
	default:
		log.Fatal("set MODE=server or MODE=client")
	}
}

func runServer(source *workloadapi.X509Source) {
	// Only this exact SPIFFE ID may connect; every other caller fails the TLS handshake.
	allowed := spiffeid.RequireFromString(env("ALLOWED_CLIENT_ID", "spiffe://example.org/client"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The caller's identity comes from its client cert, not from a password or token.
		peer, _ := x509svid.IDFromCert(r.TLS.PeerCertificates[0])
		log.Printf("request from %s", peer)
		fmt.Fprintf(w, "hello %s, the time is %s", peer, time.Now().Format(time.TimeOnly))
	})

	srv := &http.Server{
		Addr: ":8443",
		// mTLS: present our cert, require a client cert from our trust domain,
		// then check that the client's ID is the allowed one.
		TLSConfig: tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeID(allowed)),
		// Log failed handshakes, so you can see rejected callers.
		ErrorLog: log.New(os.Stderr, "tls: ", 0),
	}
	log.Printf("listening on :8443, only accepting %s", allowed)
	// Empty cert/key paths: the certs come from TLSConfig and update on every rotation.
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

func runClient(source *workloadapi.X509Source) {
	// We also check the server: it must prove it is spiffe://example.org/server.
	// This takes the place of normal hostname checks in TLS.
	serverID := spiffeid.RequireFromString(env("SERVER_ID", "spiffe://example.org/server"))
	url := env("SERVER_URL", "https://server:8443/")

	// A normal HTTP client, except its TLS config uses our SPIFFE cert.
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID)),
			// New connection (and TLS handshake) for every request. Certs are only checked
			// during a handshake, so with reused connections an expired cert goes unnoticed.
			DisableKeepAlives: true,
		},
	}

	// Call the server every 5s. The serial number in the log changes when the cert rotates.
	for {
		svid, _ := source.GetX509SVID()
		cert := svid.Certificates[0]
		serial := cert.SerialNumber.Bytes()
		mine := fmt.Sprintf("my cert serial …%x expires %s",
			serial[len(serial)-3:], cert.NotAfter.Format(time.TimeOnly))

		resp, err := client.Get(url)
		if err != nil {
			log.Printf("FAILED: %v | %s", err, mine)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("OK: %q | %s", body, mine)
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
