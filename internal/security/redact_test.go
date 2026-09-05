package security

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactor_ScrubsRegisteredSecrets(t *testing.T) {
	redactor := NewRedactor()
	const (
		token     = "ABCDEF0123456789"
		basicUser = "operator"
		basicPass = "hunter2-super-secret"
		headerEnv = "X-Header-Env-Secret-Value"
	)
	redactor.Register(token)
	redactor.Register(basicPass)
	redactor.Register(headerEnv)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: redactor.ReplaceAttr,
	}))

	logger.Info("grant admitted",
		"token", token,
		"password", basicPass,
		"user", basicUser,
		"header_env_value", headerEnv,
		"cookie", "some-cookie-value",
		"authorization", "Basic dXNlcjpwYXNz",
	)

	output := buf.String()
	for _, secret := range []string{token, basicPass, headerEnv} {
		if strings.Contains(output, secret) {
			t.Fatalf("log output contains secret %q:\n%s", secret, output)
		}
	}
	if !strings.Contains(output, "[redacted]") {
		t.Fatalf("log output does not contain the redaction placeholder:\n%s", output)
	}
}
