// Copyright Contributors to the Open Cluster Management project

package config

import (
	"context"
	"crypto/tls"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestConvertTLSVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   configv1.TLSProtocolVersion
		want    uint16
	}{
		{"TLS 1.0", configv1.VersionTLS10, tls.VersionTLS10},
		{"TLS 1.1", configv1.VersionTLS11, tls.VersionTLS11},
		{"TLS 1.2", configv1.VersionTLS12, tls.VersionTLS12},
		{"TLS 1.3", configv1.VersionTLS13, tls.VersionTLS13},
		{"unknown defaults to TLS 1.2", "VersionTLS99", tls.VersionTLS12},
		{"empty defaults to TLS 1.2", "", tls.VersionTLS12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertTLSVersion(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertCipherSuites(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []uint16
	}{
		{
			name:  "valid ECDHE ciphers",
			input: []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES256-GCM-SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "TLS 1.3 ciphers filtered out",
			input: []string{"TLS_AES_128_GCM_SHA256", "ECDHE-RSA-AES128-GCM-SHA256"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		{
			name:  "unknown ciphers skipped",
			input: []string{"UNKNOWN-CIPHER", "ECDHE-RSA-AES256-GCM-SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "RSA ciphers",
			input: []string{"AES128-GCM-SHA256", "AES256-GCM-SHA384"},
			want:  []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_RSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  nil,
		},
		{
			name:  "partial unknown logs warning but returns mapped ciphers",
			input: []string{"DHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES256-GCM-SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertCipherSuites(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertCipherSuites_AllUnmappedFallsBackToIntermediate(t *testing.T) {
	result := convertCipherSuites([]string{"DHE-RSA-AES128-GCM-SHA256", "DHE-RSA-AES256-GCM-SHA384"})
	assert.True(t, len(result) > 1, "Should fall back to Intermediate profile ciphers when no ciphers map")

	// Verify it matches the Intermediate profile
	expected := convertCipherSuites(configv1.TLSProfiles[configv1.TLSProfileIntermediateType].Ciphers)
	assert.Equal(t, expected, result)
}

func newFakeAPIServer(tlsProfile map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "config.openshift.io/v1",
			"kind":       "APIServer",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"spec": map[string]interface{}{},
		},
	}
	if tlsProfile != nil {
		obj.Object["spec"].(map[string]interface{})["tlsSecurityProfile"] = tlsProfile
	}
	return obj
}

func TestGetTLSConfig_NoProfile(t *testing.T) {
	scheme := runtime.NewScheme()
	apiServer := newFakeAPIServer(nil)
	client := dynamicfake.NewSimpleDynamicClient(scheme, apiServer)

	cfg, err := getTLSConfigWithClient(context.Background(), client)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should default to Intermediate profile (TLS 1.2)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.True(t, len(cfg.CipherSuites) > 1, "Intermediate profile should have multiple ciphers")
}

func TestGetTLSConfig_IntermediateProfile(t *testing.T) {
	scheme := runtime.NewScheme()
	apiServer := newFakeAPIServer(map[string]interface{}{
		"type": "Intermediate",
	})
	client := dynamicfake.NewSimpleDynamicClient(scheme, apiServer)

	cfg, err := getTLSConfigWithClient(context.Background(), client)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
}

func TestGetTLSConfig_OldProfile(t *testing.T) {
	scheme := runtime.NewScheme()
	apiServer := newFakeAPIServer(map[string]interface{}{
		"type": "Old",
	})
	client := dynamicfake.NewSimpleDynamicClient(scheme, apiServer)

	cfg, err := getTLSConfigWithClient(context.Background(), client)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	// Old profile allows TLS 1.0
	assert.Equal(t, uint16(tls.VersionTLS10), cfg.MinVersion)
}

func TestGetTLSConfig_CustomProfile(t *testing.T) {
	scheme := runtime.NewScheme()
	apiServer := newFakeAPIServer(map[string]interface{}{
		"type": "Custom",
		"custom": map[string]interface{}{
			"ciphers":       []interface{}{"ECDHE-RSA-AES256-GCM-SHA384"},
			"minTLSVersion": "VersionTLS13",
		},
	})
	client := dynamicfake.NewSimpleDynamicClient(scheme, apiServer)

	cfg, err := getTLSConfigWithClient(context.Background(), client)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
}

func TestGetTLSConfig_ResourceNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme) // no objects

	cfg, err := getTLSConfigWithClient(context.Background(), client)

	assert.Error(t, err)
	assert.NotNil(t, cfg)
	// Should return default fallback config (Intermediate profile)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.True(t, len(cfg.CipherSuites) > 1, "Default should use Intermediate profile ciphers")
}
