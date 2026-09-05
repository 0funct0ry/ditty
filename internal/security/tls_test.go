package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateCert writes a self-signed cert/key pair (cn as its Subject Common
// Name) to dir, returning the cert and key file paths and the parsed
// certificate.
func generateCert(t *testing.T, dir, name, cn string) (certPath, keyPath string, cert *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}

	certPath = filepath.Join(dir, name+".crt")
	keyPath = filepath.Join(dir, name+".key")

	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	defer func() { _ = certOut.Close() }()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encode cert: %v", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	defer func() { _ = keyOut.Close() }()
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		t.Fatalf("encode key: %v", err)
	}

	return certPath, keyPath, cert
}

func TestTLSConfig_Basic(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := generateCert(t, dir, "server", "ditty-test")

	cfg, err := TLSConfig(certPath, keyPath, "")
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %v, want TLS 1.2", cfg.MinVersion)
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != "http/1.1" {
		t.Fatalf("NextProtos = %v, want [http/1.1]", cfg.NextProtos)
	}
	if cfg.ClientCAs != nil || cfg.ClientAuth != tls.NoClientCert {
		t.Fatal("TLSConfig without --client-ca must not require a client certificate")
	}
}

func TestTLSConfig_MutualTLS(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, _ := generateCert(t, dir, "server", "ditty-test")
	caPath, _, _ := generateCert(t, dir, "ca", "ditty-ca")

	cfg, err := TLSConfig(certPath, keyPath, caPath)
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth = %v, want RequireAndVerifyClientCert", cfg.ClientAuth)
	}
	if cfg.ClientCAs == nil {
		t.Fatal("ClientCAs = nil, want the parsed --client-ca pool")
	}
}

func TestTLSConfig_Errors(t *testing.T) {
	dir := t.TempDir()
	if _, err := TLSConfig(filepath.Join(dir, "missing.crt"), filepath.Join(dir, "missing.key"), ""); err == nil {
		t.Fatal("TLSConfig(missing cert/key) = nil error, want error")
	}

	certPath, keyPath, _ := generateCert(t, dir, "server", "ditty-test")
	junkCA := filepath.Join(dir, "junk-ca.pem")
	if err := os.WriteFile(junkCA, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write junk CA: %v", err)
	}
	if _, err := TLSConfig(certPath, keyPath, junkCA); err == nil {
		t.Fatal("TLSConfig(unparseable --client-ca) = nil error, want error")
	}
	if _, err := TLSConfig(certPath, keyPath, filepath.Join(dir, "missing-ca.pem")); err == nil {
		t.Fatal("TLSConfig(missing --client-ca file) = nil error, want error")
	}
}

func TestMTLSGrant_Authenticate(t *testing.T) {
	dir := t.TempDir()
	_, _, cert := generateCert(t, dir, "client", "alice")
	grant := NewMTLSGrant()

	withCert := httptest.NewRequest(http.MethodGet, "/", nil)
	withCert.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	if id, ok := grant.Authenticate(withCert); !ok || id.Label != "alice" {
		t.Fatalf("Authenticate(client cert) = %+v, %v, want alice, true", id, ok)
	}

	noTLS := &http.Request{}
	if _, ok := grant.Authenticate(noTLS); ok {
		t.Fatal("Authenticate(no TLS) = true, want false")
	}
}
