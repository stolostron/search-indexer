// Copyright Contributors to the Open Cluster Management project

package config

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCipherSuitesFromNames(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []uint16
	}{
		{
			name:  "valid ECDHE ciphers",
			input: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "unknown ciphers skipped",
			input: []string{"UNKNOWN_CIPHER", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "all unknown returns nil",
			input: []string{"UNKNOWN_1", "UNKNOWN_2"},
			want:  nil,
		},
		{
			name:  "empty input",
			input: []string{},
			want:  nil,
		},
		{
			name:  "whitespace trimmed",
			input: []string{" TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 ", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "empty strings filtered",
			input: []string{"", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", ""},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		{
			name:  "insecure ciphers resolve",
			input: []string{"TLS_RSA_WITH_AES_128_CBC_SHA"},
			want:  []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cipherSuitesFromNames(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTLSConfig_Defaults(t *testing.T) {
	t.Setenv("TLS_MIN_VERSION", "")
	t.Setenv("TLS_CIPHERS", "")

	cfg := GetTLSConfig()

	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.Nil(t, cfg.CipherSuites, "No env var should leave CipherSuites nil (Go defaults)")
}

func TestGetTLSConfig_WithEnvVars(t *testing.T) {
	t.Setenv("TLS_MIN_VERSION", "772") // TLS 1.3
	t.Setenv("TLS_CIPHERS", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384")

	cfg := GetTLSConfig()

	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	assert.Equal(t, []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	}, cfg.CipherSuites)
}

func TestGetTLSConfig_InvalidMinVersion(t *testing.T) {
	t.Setenv("TLS_MIN_VERSION", "not-a-number")
	t.Setenv("TLS_CIPHERS", "")

	cfg := GetTLSConfig()

	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion, "Should fall back to TLS 1.2")
}

func TestGetTLSConfig_InvalidCiphers(t *testing.T) {
	t.Setenv("TLS_MIN_VERSION", "")
	t.Setenv("TLS_CIPHERS", "TOTALLY_FAKE_CIPHER")

	cfg := GetTLSConfig()

	assert.Empty(t, cfg.CipherSuites, "Unresolvable ciphers should result in empty list")
}
