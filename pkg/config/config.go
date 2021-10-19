package config

import (
	"encoding/json"
	"os"
	"strconv"

	"k8s.io/klog/v2"
)

const (
	AGGREGATOR_API_VERSION     = "2.5.0"
	DEFAULT_AGGREGATOR_ADDRESS = ":3010"
	DEFAULT_DB_HOST            = "localhost"
	DEFAULT_DB_PORT            = 5432
	DEFAULT_DB_NAME            = ""
	DEFAULT_DB_USER            = ""
	DEFAULT_HTTP_TIMEOUT       = 300000 // 5 min

	// DEFAULT_EDGE_BUILD_RATE_MS      = 15000  // 15 sec
	// DEFAULT_REDISCOVER_RATE_MS      = 300000 // 5 min
	// DEFAULT_REQUEST_LIMIT           = 10    // Max number of concurrent requests.
	// DEFAULT_SKIP_CLUSTER_VALIDATION = "false"
)

// Struct to hold our configuratioin
type Config struct {
	AggregatorAddress string // address for collector <-> aggregator
	DBHost            string
	DBPort            int
	DBName            string
	DBUser            string
	DBPass            string
	HTTPTimeout       int // timeout when the http server should drop connections
	Version           string
	// EdgeBuildRateMS       int    // rate at which intercluster edges should be build
	// KubeConfig            string // Local kubeconfig path
	// RequestLimit          int    // Max number of concurrent requests. Used to prevent from overloading Redis.
	// SkipClusterValidation string // Skips cluster validation. Intended only for performance tests.
}

func New() *Config {
	return &Config{
		AggregatorAddress: getEnv("AGGREGATOR_ADDRESS", DEFAULT_AGGREGATOR_ADDRESS),
		DBHost:            getEnv("DB_HOST", DEFAULT_DB_HOST),
		DBPort:            getEnvAsInt("DB_PORT", DEFAULT_DB_PORT),
		DBName:            getEnv("DB_NAME", DEFAULT_DB_NAME),
		DBUser:            getEnv("DB_USER", DEFAULT_DB_USER),
		DBPass:            getEnv("DB_PASS", ""),
		HTTPTimeout:       getEnvAsInt("HTTP_TIMEOUT", DEFAULT_HTTP_TIMEOUT),
		Version:           AGGREGATOR_API_VERSION,
		// EdgeBuildRateMS       int    // rate at which intercluster edges should be build
		// KubeConfig            string // Local kubeconfig path
		// RedisWatchRate        int    // rate at which Redis Ping hapens to check health
		// RediscoverRateMS      int    // time in MS we should check on cluster resource type
		// RequestLimit          int    // Max number of concurrent requests. Used to prevent from overloading Redis.
		// SkipClusterValidation string // Skips cluster validation. Intended only for performance tests.
	}
}

// Format and print to logger.
func (cfg *Config) PrintConfig() {
	// Make a copy to redact secrets and sensitive information.
	tmp := cfg
	tmp.DBPass = "[REDACTED]"

	// Convert to JSON for nicer formatting.
	cfgJSON, err := json.MarshalIndent(tmp, "", "\t")
	if err != nil {
		klog.Warning("Encountered a problem formatting configuration. ", err)
		klog.Infof("Configuration %#v\n", tmp)
	}
	klog.Infof("Configuration:\n%s\n", string(cfgJSON))
}

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

// Simple helper function to read an environment variable into integer or return a default value
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}

// Helper to read an environment variable into a bool or return default value
// func getEnvAsBool(name string, defaultVal bool) bool {
// 	valStr := getEnv(name, "")
// 	if val, err := strconv.ParseBool(valStr); err == nil {
// 		return val
// 	}

// 	return defaultVal
// }

// Helper to read an environment variable into a string slice or return default value
// func getEnvAsSlice(name string, defaultVal []string, sep string) []string {
// 	valStr := getEnv(name, "")

// 	if valStr == "" {
// 		return defaultVal
// 	}

// 	val := strings.Split(valStr, sep)

// 	return val
// }
