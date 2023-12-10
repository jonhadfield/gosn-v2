package common

import (
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// API.
	APIServer = "https://api.standardnotes.com"
	SyncPath  = "/items/sync" // remote path for making sync calls
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
	RequestTimeout     = 60  // HTTP transport limit
	ConnectionTimeout  = 10  // HTTP transport dialer limit
	KeepAliveTimeout   = 10  // HTTP transport dialer limit
)

func NewHTTPClient() *retryablehttp.Client {
	c := retryablehttp.NewClient()
	c.RetryMax = 3
	c.HTTPClient.Timeout = RequestTimeout * time.Second
	c.Logger = nil

	return c
}
