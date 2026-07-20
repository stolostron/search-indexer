// Copyright Contributors to the Open Cluster Management project

package config

import (
	"crypto/tls"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// GetTLSConfig builds a *tls.Config from environment variables set by the operator.
//
// Expected env vars (set by the search-v2-operator from the cluster's APIServer TLS profile):
//   - TLS_MIN_VERSION: uint16 value as string (e.g. "771" for TLS 1.2, "772" for TLS 1.3)
//   - TLS_CIPHERS: comma-separated IANA cipher suite names
//     (e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384")
//
// If the env vars are not set (development mode, non-operator deployment), defaults to
// TLS 1.2 with Go's default cipher suite selection.
func GetTLSConfig() *tls.Config {
	minVersion := defaultMinVersion
	cipherSuites := []uint16(nil)

	if v := os.Getenv("TLS_MIN_VERSION"); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			klog.Warningf("Invalid TLS_MIN_VERSION %q, using default TLS 1.2: %v", v, err)
		} else {
			minVersion = uint16(parsed)
			// e.g. (770&0xff) - 1 = 2 - 1 = 1 -> TLS 1.1, (769&0xff) - 1 = 1 - 1 = 0 -> TLS 1.0
			klog.Infof("TLS min version from env: TLS 1.%d", (minVersion&0xff)-1)
		}
	}

	if v := os.Getenv("TLS_CIPHERS"); v != "" {
		names := strings.Split(v, ",")
		cipherSuites = cipherSuitesFromNames(names)
		if len(cipherSuites) == 0 {
			klog.Warning("No valid cipher suites resolved from TLS_CIPHERS, using Go defaults")
		} else {
			klog.Infof("TLS cipher suites from env: %d configured", len(cipherSuites))
		}
	}

	return &tls.Config{
		MinVersion:   minVersion,
		CipherSuites: cipherSuites,
	}
}

const defaultMinVersion uint16 = tls.VersionTLS12

// cipherSuitesFromNames resolves IANA cipher suite names to crypto/tls uint16 IDs
// using Go's stdlib. No hardcoded map — automatically picks up new ciphers when Go adds them.
func cipherSuitesFromNames(names []string) []uint16 {
	lookup := map[string]uint16{}
	for _, cs := range tls.CipherSuites() {
		lookup[cs.Name] = cs.ID
	}
	for _, cs := range tls.InsecureCipherSuites() {
		lookup[cs.Name] = cs.ID
	}

	var result []uint16
	var unknown []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if id, ok := lookup[name]; ok {
			result = append(result, id)
		} else {
			unknown = append(unknown, name)
		}
	}

	if len(unknown) > 0 {
		klog.Warningf("TLS cipher suites not recognized by Go, skipped: %v", unknown)
	}

	return result
}
