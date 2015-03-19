// HTTP server that uses OAuth to create security.Blessings objects.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"v.io/v23"
	"v.io/x/lib/vlog"

	_ "v.io/x/ref/profiles/static"
	"v.io/x/ref/services/identity/internal/auditor"
	"v.io/x/ref/services/identity/internal/blesser"
	"v.io/x/ref/services/identity/internal/caveats"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/services/identity/internal/server"
	"v.io/x/ref/services/identity/internal/util"
)

var (
	// Flags controlling the HTTP server
	host      = flag.String("host", "localhost", "Hostname the HTTP server listens on. This can be the name of the host running the webserver, but if running behind a NAT or load balancer, this should be the host name that clients will connect to. For example, if set to 'x.com', Vanadium identities will have the IssuerName set to 'x.com' and clients can expect to find the root name and public key of the signer at 'x.com/blessing-root'.")
	httpaddr  = flag.String("httpaddr", "localhost:0", "Address on which the HTTP server listens on.")
	tlsconfig = flag.String("tlsconfig", "", "Comma-separated list of TLS certificate and private key files, in that order. This must be provided.")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	// Duration to use for tls cert and blessing duration.
	duration := 365 * 24 * time.Hour

	// If no tlsconfig has been provided, write and use our own.
	if flag.Lookup("tlsconfig").Value.String() == "" {
		certFile, keyFile, err := util.WriteCertAndKey(*host, duration)
		if err != nil {
			vlog.Fatal(err)
		}
		if err := flag.Set("tlsconfig", certFile+","+keyFile); err != nil {
			vlog.Fatal(err)
		}
	}

	auditor, reader := auditor.NewMockBlessingAuditor()
	revocationManager := revocation.NewMockRevocationManager()
	oauthProvider := oauth.NewMockOAuth()

	params := blesser.OAuthBlesserParams{
		OAuthProvider:     oauthProvider,
		BlessingDuration:  duration,
		RevocationManager: revocationManager,
	}

	ctx, shutdown := v23.Init()
	defer shutdown()

	listenSpec := v23.GetListenSpec(ctx)
	s := server.NewIdentityServer(
		oauthProvider,
		auditor,
		reader,
		revocationManager,
		params,
		caveats.NewMockCaveatSelector(),
		nil)
	s.Serve(ctx, &listenSpec, *host, *httpaddr, *tlsconfig)
}

func usage() {
	fmt.Fprintf(os.Stderr, `%s starts a test version of the identityd server that
mocks out oauth, auditing, and revocation.

To generate TLS certificates so the HTTP server can use SSL:
go run $(go list -f {{.Dir}} "crypto/tls")/generate_cert.go --host <IP address>

Flags:
`, os.Args[0])
	flag.PrintDefaults()
}
