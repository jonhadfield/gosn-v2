package common

import (
	"fmt"
	"os"
	"strconv"
)

const (
	EnvPostSyncRequestDelay = "SN_POST_SYNC_REQUEST_DELAY"
	EnvPostSignInDelay      = "SN_POST_SIGN_IN_DELAY"
	EnvSchemaValidation     = "SN_SCHEMA_VALIDATION"
	EnvServer               = "SN_SERVER"
	EnvEmail                = "SN_EMAIL"
	EnvPassword             = "SN_PASSWORD"
	EnvSkipSessionTests     = "SN_SKIP_SESSION_TESTS"
	EnvDebug                = "SN_DEBUG"
	EnvRequestTimeout       = "SN_REQUEST_TIMEOUT"  // Override default request timeout in seconds
	EnvRetryWaitMin         = "SN_RETRY_WAIT_MIN"   // Override minimum retry wait in seconds
	EnvRetryWaitMax         = "SN_RETRY_WAIT_MAX"   // Override maximum retry wait in seconds
)

// ParseEnvInt64 looks up an environment variable and attempts to parse
// it as an int64. It returns the parsed value, a boolean indicating
// whether the variable was set, and any error encountered.
func ParseEnvInt64(name string) (int64, bool, error) {
	val := os.Getenv(name)
	if val == "" {
		return 0, false, nil
	}

	v, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid %s value: %w", name, err)
	}

	return v, true, nil
}
