// Copyright Contributors to the Open Cluster Management project

package config

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

// Should use default value when environment variable does not exist.
func Test_getEnv_default(t *testing.T) {
	res := getEnv("ENV_VARIABLE_NOT_DEFINED", "default-value")

	if res != "default-value" {
		t.Errorf("Failed testing getEnv()  Expected: %s  Got: %s", "default-value", res)
	}
}

// Should load string value from environment.
func Test_getEnv(t *testing.T) {
	_ = os.Setenv("TEST_VARIABLE", "test-value")
	res := getEnv("TEST_VARIABLE", "default-value")

	if res != "test-value" {
		t.Errorf("Failed testing getEnv()  Expected: %s  Got: %s", "test-value", res)
	}
}

// Should use default value when environment variable does not exist.
func Test_getEnvAsInt_default(t *testing.T) {
	res := getEnvAsInt("ENV_VARIABLE_NOT_DEFINED", 99)

	if res != 99 {
		t.Errorf("Failed testing getEnvAsIInt() Expected: %d  Got: %d", 99, res)
	}
}

// Should load int value from environment.
func Test_getEnvAsInt(t *testing.T) {
	_ = os.Setenv("TEST_VARIABLE", "99")
	res := getEnvAsInt("TEST_VARIABLE", 0)

	if res != 99 {
		t.Errorf("Failed testing getEnv()  Expected: %d  Got: %d", 99, res)
	}
}

// Should print environment and redact the database password.
func Test_PrintConfig(t *testing.T) {
	// Redirect the logger output.
	var buf bytes.Buffer
	klog.LogToStderr(false)
	klog.SetOutput(&buf)
	defer func() {
		klog.SetOutput(os.Stderr)
	}()

	// Call the function.
	c := new()
	c.PrintConfig()

	// Verify environment was logged as expected.
	logMsg := buf.String()
	if !strings.Contains(logMsg, "\"DBPass\": \"[REDACTED]\"") {
		t.Error("Expected password to be redacted when logging configuration")
	}

	// Verify that the config wasn't changed when redacting the password.
	if c.DBPass == "[REDACTED]" {
		t.Error("Expected config.DBPass to not be permanently changed when redacting password.")
	}
}

// Should validate that DB_NAME, DB_USER, and DB_PASS are required environment variables.
func Test_Validate(t *testing.T) {
	_ = os.Setenv("DB_NAME", "test")
	_ = os.Setenv("DB_USER", "test")
	_ = os.Setenv("DB_PASS", "test")
	conf := new()

	result := conf.Validate()
	if result != nil {
		t.Errorf("Expected %v Got: %+v", nil, result)
	}

	_ = os.Setenv("DB_PASS", "")
	conf = new()
	result = conf.Validate()
	if result.Error() != "Required environment DB_PASS is not set." {
		t.Errorf("Expected %s Got: %s", "Required environment DB_PASS is not set.", result)
	}

	_ = os.Setenv("DB_USER", "")
	conf = new()
	result = conf.Validate()
	if result.Error() != "Required environment DB_USER is not set." {
		t.Errorf("Expected %s Got: %s", "Required environment DB_USER is not set.", result)
	}

	_ = os.Setenv("DB_NAME", "")
	conf = new()
	result = conf.Validate()
	if result.Error() != "Required environment DB_NAME is not set." {
		t.Errorf("Expected %s Got: %s", "Required environment DB_NAME is not set.", result)
	}
}
