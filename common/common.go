package common

import (
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// API.
	APIServer = "https://api.standardnotes.com"
	SyncPath  = "/items/sync" // remote path for making sync calls

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
	SNItemTypeExtensionRepo        = "SN|File"
	SNItemTypeFile                 = "SN|ExtensionRepo"

	// Authentication.
	AuthParamsPath    = "/v2/login-params" // remote path for getting auth parameters
	AuthRegisterPath  = "/v1/users"        // remote path for registering user
	AuthRefreshPath   = "/v1/sessions/refresh"
	SignInPath        = "/v2/login" // remote path for authenticating
	MinPasswordLength = 8           // minimum password length when registering
	// PageSize is the maximum number of items to return with each call.
	PageSize            = 150
	TimeLayout          = "2006-01-02T15:04:05.000Z"
	TimeLayout2         = "2006-01-02T15:04:05.000000Z"
	DefaultSNVersion    = "004"
	DefaultPasswordCost = 110000

	// LOGGING.
	LibName       = "gosn-v2" // name of library used in logging
	MaxDebugChars = 120       // number of characters to display when logging API response body

	// HTTP.
	MaxIdleConnections = 100 // HTTP transport limit
	RequestTimeout     = 30  // HTTP transport limit
	ConnectionTimeout  = 3   // HTTP transport dialer limit
	KeepAliveTimeout   = 60  // HTTP transport dialer limit
	MaxRequestRetries  = 3
)

func NewHTTPClient() *retryablehttp.Client {
	c := retryablehttp.NewClient()

	t := http.DefaultTransport.(*http.Transport).Clone()

	envProxyUrl := os.Getenv("HTTP_PROXY")

	if envProxyUrl != "" {
		proxyUrl, err := url.Parse(envProxyUrl)
		if err != nil {
			log.Fatalf("HTTP_PROXY url %s invalid\n", proxyUrl)
		}

		t.Proxy = http.ProxyURL(proxyUrl)
	}

	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	c.HTTPClient.Transport = t

	c.RetryMax = MaxRequestRetries
	c.RetryWaitMin = 60 * time.Second
	c.RetryWaitMax = 180 * time.Second
	// c.Backoff = retryablehttp.LinearJitterBackoff(backoff)
	c.Backoff = backoff
	// c.Backoff = retryablehttp.DefaultBackoff
	c.HTTPClient.Timeout = RequestTimeout * time.Second
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
