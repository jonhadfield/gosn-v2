package gosn

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	// API
	apiServer        = "https://sync.standardnotes.org"
	authParamsPath   = "/auth/params"  // remote path for getting auth parameters
	authRegisterPath = "/auth"         // remote path for registering user
	signInPath       = "/auth/sign_in" // remote path for authenticating
	syncPath         = "/items/sync"   // remote path for making sync calls
	// PageSize is the maximum number of items to return with each call
	PageSize            = 300
	timeLayout          = "2006-01-02T15:04:05.000Z"
	timeLayout2         = "2006-01-02T15:04:05.000000Z"
	defaultSNVersion    = "004"
	defaultPasswordCost = 110000
	numRawSessionTokens = 7

	// LOGGING
	libName       = "gosn" // name of library used in logging
	maxDebugChars = 120    // number of characters to display when logging API response body

	// HTTP
	maxIdleConnections = 100 // HTTP transport limit
	requestTimeout     = 60  // HTTP transport limit
	connectionTimeout  = 10  // HTTP transport dialer limit
	keepAliveTimeout   = 10  // HTTP transport dialer limit
)

var (
	httpClient *http.Client
)

func init() {
	httpClient = createHTTPClient()
}

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: maxIdleConnections,
			DisableKeepAlives:   false,
			DisableCompression:  false,
			DialContext: (&net.Dialer{
				Timeout:   connectionTimeout * time.Second,
				KeepAlive: keepAliveTimeout * time.Second,
			}).DialContext,
		},
		Timeout: time.Duration(requestTimeout) * time.Second,
	}
}

func debugPrint(show bool, msg string) {
	if show {
		if len(msg) > maxDebugChars {
			msg = msg[:maxDebugChars] + "..."
		}

		log.Println(libName, "|", msg)
	}
}

func show(msg string, item interface{}) {
	fmt.Printf("%s: %+v\n", msg, item)
}
func showlist(msg string, items []interface{}) {
	for x, item := range items {
		fmt.Printf("%s %d: %+v\n", msg, x, item)
	}
}
