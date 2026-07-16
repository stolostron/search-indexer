// Copyright Contributors to the Open Cluster Management project

package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

var apiServerGVR = schema.GroupVersionResource{
	Group:    "config.openshift.io",
	Version:  "v1",
	Resource: "apiservers",
}

// GetTLSConfig reads the TLS security profile from the OpenShift APIServer resource
// and returns a *tls.Config. If the APIServer resource cannot be read (e.g., not on
// OpenShift or in development mode), it returns a default TLS 1.2 config.
func GetTLSConfig(ctx context.Context) (*tls.Config, error) {
	dynamicClient, err := dynamic.NewForConfig(getKubeConfig())
	if err != nil {
		return defaultTLSConfig(), fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return getTLSConfigWithClient(ctx, dynamicClient)
}

func getTLSConfigWithClient(ctx context.Context, dynamicClient dynamic.Interface) (*tls.Config, error) {
	obj, err := dynamicClient.Resource(apiServerGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return defaultTLSConfig(), fmt.Errorf("failed to get APIServer resource: %w", err)
	}

	// Extract spec.tlsSecurityProfile from unstructured object
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		klog.Info("APIServer spec not found, using Intermediate TLS profile")
		return tlsConfigFromProfile(configv1.TLSProfiles[configv1.TLSProfileIntermediateType]), nil
	}

	profileRaw, exists := spec["tlsSecurityProfile"]
	if !exists || profileRaw == nil {
		klog.Info("No TLS security profile set, using Intermediate TLS profile")
		return tlsConfigFromProfile(configv1.TLSProfiles[configv1.TLSProfileIntermediateType]), nil
	}

	// Marshal and unmarshal to convert unstructured to typed TLSSecurityProfile
	profileBytes, err := json.Marshal(profileRaw)
	if err != nil {
		return defaultTLSConfig(), fmt.Errorf("failed to marshal TLS profile: %w", err)
	}

	var profile configv1.TLSSecurityProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		return defaultTLSConfig(), fmt.Errorf("failed to unmarshal TLS profile: %w", err)
	}

	// For preset profiles, look up the predefined spec
	if profileSpec, ok := configv1.TLSProfiles[profile.Type]; ok {
		klog.Infof("Using %s TLS profile", profile.Type)
		return tlsConfigFromProfile(profileSpec), nil
	}

	// For custom profile, use the inline spec
	if profile.Type == configv1.TLSProfileCustomType && profile.Custom != nil {
		klog.Info("Using Custom TLS profile")
		return tlsConfigFromProfile(&profile.Custom.TLSProfileSpec), nil
	}

	// Fallback to Intermediate
	klog.Warning("Unrecognized TLS profile type, falling back to Intermediate")
	return tlsConfigFromProfile(configv1.TLSProfiles[configv1.TLSProfileIntermediateType]), nil
}

func tlsConfigFromProfile(profileSpec *configv1.TLSProfileSpec) *tls.Config {
	minVersion := convertTLSVersion(profileSpec.MinTLSVersion)
	cipherSuites := convertCipherSuites(profileSpec.Ciphers)

	klog.Infof("TLS config: minVersion=%s, cipherSuites=%d", profileSpec.MinTLSVersion, len(cipherSuites))

	return &tls.Config{
		MinVersion:   minVersion,
		CipherSuites: cipherSuites,
	}
}

// defaultTLSConfig returns a fallback TLS config for when the APIServer profile
// cannot be read (development mode, non-OpenShift clusters).
// Uses Intermediate profile ciphers as a secure default.
func defaultTLSConfig() *tls.Config {
	klog.Warning("Using default TLS config (Intermediate profile)")
	return tlsConfigFromProfile(configv1.TLSProfiles[configv1.TLSProfileIntermediateType])
}

// convertTLSVersion converts an OpenShift TLSProtocolVersion to a crypto/tls version constant.
func convertTLSVersion(version configv1.TLSProtocolVersion) uint16 {
	switch version {
	case configv1.VersionTLS10:
		return tls.VersionTLS10
	case configv1.VersionTLS11:
		return tls.VersionTLS11
	case configv1.VersionTLS12:
		return tls.VersionTLS12
	case configv1.VersionTLS13:
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12
	}
}

// convertCipherSuites converts OpenSSL cipher suite names to crypto/tls uint16 constants.
// TLS 1.3 cipher suites are managed automatically by Go and are filtered out.
func convertCipherSuites(cipherNames []string) []uint16 {
	cipherMap := map[string]uint16{
		// TLS 1.2 ECDHE ciphers (GCM and ChaCha20)
		"ECDHE-RSA-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-RSA-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-ECDSA-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-ECDSA-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-RSA-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-ECDSA-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,

		// TLS 1.2 ECDHE ciphers (CBC)
		"ECDHE-RSA-AES128-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-RSA-AES128-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"ECDHE-ECDSA-AES128-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-ECDSA-AES128-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"ECDHE-RSA-AES256-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"ECDHE-ECDSA-AES256-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,

		// RSA ciphers
		"AES128-GCM-SHA256": tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"AES256-GCM-SHA384": tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"AES128-SHA256":     tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		"AES128-SHA":        tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"AES256-SHA":        tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"DES-CBC3-SHA":      tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}

	var result []uint16
	for _, name := range cipherNames {
		// Skip TLS 1.3 cipher suites (auto-managed by Go)
		if len(name) > 4 && name[:4] == "TLS_" {
			continue
		}
		if cipher, ok := cipherMap[name]; ok {
			result = append(result, cipher)
		}
	}

	return result
}
