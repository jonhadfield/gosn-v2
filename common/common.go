package common

import (
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// API.
	APIServer  = "https://api.standardnotes.com"
	APIVersion = "20240226"  // API version used in requests (latest version with cookie support)
	SyncPath   = "/v1/items" // remote path for making sync calls

	// Type names.
	SNItemTypeNote                 = "Note"
	SNItemTypeTag                  = "Tag"
	SNItemTypeComponent            = "SN|Component"
	SNItemTypeItemsKey             = "SN|ItemsKey"
	SNItemTypeTheme                = "SN|Theme"
	SNItemTypePrivileges           = "SN|Privileges"
	SNItemTypeExtension            = "Extension"
	SNItemTypeSFExtension          = "SF|Extension"
	SNItemTypeSFMFA                = "SF|MFA"
	SNItemTypeSmartTag             = "SN|SmartTag"
	SNItemTypeFileSafeFileMetaData = "SN|FileSafe|FileMetadata"
	SNItemTypeFileSafeIntegration  = "SN|FileSafe|Integration"
	SNItemTypeFileSafeCredentials  = "SN|FileSafe|Credentials" //nolint:gosec
	SNItemTypeUserPreferences      = "SN|UserPreferences"
	SNItemTypeExtensionRepo        = "SN|ExtensionRepo"
	SNItemTypeFile                 = "SN|File"
	// New item types for modern Standard Notes features
	SNItemTypeTrustedContact       = "SN|TrustedContact"
	SNItemTypeVaultListing         = "SN|VaultListing"
	SNItemTypeKeySystemRootKey     = "SN|KeySystemRootKey"
	SNItemTypeKeySystemItemsKey    = "SN|KeySystemItemsKey"

	// Authentication.
	AuthParamsPath    = "/v2/login-params" // remote path for getting auth parameters
	AuthRegisterPath  = "/v1/users"        // remote path for registering user
	AuthRefreshPath   = "/v1/sessions/refresh"
	SignInPath        = "/v2/login" // remote path for authenticating
	MinPasswordLength = 8           // minimum password length when registering
	// PageSize is the maximum number of items to return with each call.
	PageSize            = 150
	// Dynamic batch sizing parameters
	MaxPageSize         = 500              // Maximum items per batch
	MinPageSize         = 50               // Minimum items per batch
	TargetPayloadSize   = 256 * 1024       // Target 256KB per request
	// MinSyncInterval is the minimum time between sync operations when no changes exist
	MinSyncInterval     = 5 * time.Minute
	// Sync token TTL settings
	SyncTokenMaxAge     = 24 * time.Hour  // Maximum age before token expires
	SyncTokenSoftAge    = 12 * time.Hour  // Age when warning is logged
	TimeLayout          = "2006-01-02T15:04:05.000Z"
	TimeLayout2         = "2006-01-02T15:04:05.000000Z"
	DefaultSNVersion    = "004"
	DefaultPasswordCost = 110000

	// LOGGING.
	LibName       = "gosn-v2" // name of library used in logging
	MaxDebugChars = 120       // number of characters to display when logging API response body

	// HTTP.
	MaxIdleConnections = 100 // HTTP transport limit
	RequestTimeout     = 30  // HTTP transport limit - increased from 5 to handle large syncs
	ConnectionTimeout  = 10  // HTTP transport dialer limit - increased from 3
	KeepAliveTimeout   = 60  // HTTP transport dialer limit
	MaxRequestRetries  = 5
)

func NewHTTPClient() *retryablehttp.Client {
	c := retryablehttp.NewClient()

	// Allow overriding timeout via environment variable
	timeout := RequestTimeout
	if envTimeout, ok, err := ParseEnvInt64(EnvRequestTimeout); err == nil && ok {
		timeout = int(envTimeout)
	}

	// Add cookie jar for automatic cookie handling (API version 20240226)
	//
	// ⚠️  THREAD-SAFETY WARNING ⚠️
	// Go's http.CookieJar is NOT thread-safe for concurrent requests.
	//
	// This affects:
	// - Cookie-based authentication (tokens with "2:" prefix)
	// - Concurrent sync operations on the same session
	// - Sharing session.HTTPClient across goroutines
	//
	// Mitigation: items.syncMutex serializes sync requests to prevent races.
	//
	// Safe usage:
	//   - Use separate Session instances for concurrent operations
	//   - Serialize requests to the same session with mutex
	//
	// See claudedocs/thread_safety.md for detailed guidance.
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Failed to create cookie jar: %v\n", err)
	}
	c.HTTPClient.Jar = jar

	t := http.DefaultTransport.(*http.Transport).Clone()

	envProxyUrl := os.Getenv("HTTP_PROXY")

	if envProxyUrl != "" {
		proxyUrl, err := url.Parse(envProxyUrl)
		if err != nil {
			log.Fatalf("HTTP_PROXY url %s invalid\n", proxyUrl)
		}

		t.Proxy = http.ProxyURL(proxyUrl)
	}

	t.MaxIdleConns = MaxIdleConnections
	t.MaxConnsPerHost = MaxIdleConnections
	t.MaxIdleConnsPerHost = MaxIdleConnections
	c.HTTPClient.Transport = t

	c.RetryMax = MaxRequestRetries

	// Allow overriding retry wait times via environment variables
	retryWaitMin := 2
	if envMin, ok, err := ParseEnvInt64(EnvRetryWaitMin); err == nil && ok {
		retryWaitMin = int(envMin)
	}
	retryWaitMax := 5
	if envMax, ok, err := ParseEnvInt64(EnvRetryWaitMax); err == nil && ok {
		retryWaitMax = int(envMax)
	}

	c.RetryWaitMin = time.Duration(retryWaitMin) * time.Second
	c.RetryWaitMax = time.Duration(retryWaitMax) * time.Second
	// c.Backoff = retryablehttp.LinearJitterBackoff(backoff)
	c.Backoff = backoff
	// c.Backoff = retryablehttp.DefaultBackoff
	c.HTTPClient.Timeout = time.Duration(timeout) * time.Second
	c.Logger = nil

	return c
}

func backoff(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	if resp != nil {
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			// SN API doesn't currently return Retry-After header but retain in case it does in future
			if s, ok := resp.Header["Retry-After"]; ok {
				if sleep, err := strconv.ParseInt(s[0], 10, 64); err == nil {
					return time.Second * time.Duration(sleep)
				}
			}
		}
	}

	mult := math.Pow(2, float64(attemptNum)) * float64(min)
	sleep := time.Duration(mult)

	if float64(sleep) != mult || sleep > max {
		sleep = max
	}

	return sleep
}

const HeaderContentType = "Content-Type"

const (
	SNAPIContentType = "application/json"
)
