package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// TLSConfig builds the *tls.Config for --tls-cert/--tls-key (SPEC.md §6.3):
// TLS 1.2 minimum, a modern cipher suite list, and HTTP/1.1 only (no h2 over
// this listener). When clientCAPath is non-empty, it additionally requires
// and verifies a client certificate against that CA (mTLS, --client-ca).
func TLSConfig(certFile, keyFile, clientCAPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("security: load --tls-cert/--tls-key: %w", err)
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"http/1.1"},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	if clientCAPath != "" {
		pem, err := os.ReadFile(clientCAPath)
		if err != nil {
			return nil, fmt.Errorf("security: read --client-ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("security: --client-ca %q contains no usable certificate", clientCAPath)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg, nil
}

// MTLSGrant admits any request presenting a client certificate verified by
// the server's TLS listener (ClientAuth: RequireAndVerifyClientCert, set by
// TLSConfig when --client-ca is given). Its Subject Common Name becomes the
// Client label (SPEC.md §6.2).
type MTLSGrant struct{}

// NewMTLSGrant returns the mTLS Grant. It carries no state of its own: the
// actual verification happens in the TLS handshake, driven by the
// *tls.Config TLSConfig built with a non-empty clientCAPath.
func NewMTLSGrant() *MTLSGrant { return &MTLSGrant{} }

func (g *MTLSGrant) Authenticate(r *http.Request) (Identity, bool) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return Identity{}, false
	}
	cn := r.TLS.PeerCertificates[0].Subject.CommonName
	if cn == "" {
		return Identity{}, false
	}
	return Identity{Label: cn, Method: "mtls"}, true
}
