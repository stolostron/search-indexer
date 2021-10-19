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

	os.Setenv("TEST_VARIABLE", "test-value")
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

	os.Setenv("TEST_VARIABLE", "99")
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
	c := New()
	c.PrintConfig()

	// Validate environment was logged as expeected.
	logMsg := buf.String()
	if !strings.Contains(logMsg, "\"DBPass\": \"[REDACTED]\"") {
		t.Error("Expected password to be redacted when logging configuration")
	}
}
