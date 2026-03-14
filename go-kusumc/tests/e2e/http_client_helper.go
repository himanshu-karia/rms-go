package e2e

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// httpClient returns an HTTP client that can talk to self-signed HTTPS endpoints when enabled via env.
// HTTP_TLS_INSECURE=true skips verification; HTTP_CA_PATH adds a custom CA bundle.
func httpClient(t testing.TB) *http.Client {
	t.Helper()

	tlsCfg, err := httpTLSConfigFromEnv()
	if err != nil {
		t.Fatalf("http TLS config: %v", err)
	}

	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		Timeout:   15 * time.Second,
	}
}

func httpTLSConfigFromEnv() (*tls.Config, error) {
	caPath := os.Getenv("HTTP_CA_PATH")
	insecure := strings.EqualFold(os.Getenv("HTTP_TLS_INSECURE"), "true")

	cfg := &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // test-only opt-in
	if caPath == "" {
		return cfg, nil
	}

	pem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read HTTP_CA_PATH: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("append CA certs")
	}
	cfg.RootCAs = pool

	return cfg, nil
}
