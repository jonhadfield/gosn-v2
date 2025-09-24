package auth

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/log"
)

type cryptoSource struct{}

func (s cryptoSource) Seed(seed int64) {}

func (s cryptoSource) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s cryptoSource) Uint64() (v uint64) {
	err := binary.Read(crand.Reader, binary.BigEndian, &v)
	if err != nil {
		log.Fatal(err.Error())
	}

	return v
}

type doAuthRequestOutput struct {
	Data     AuthParamsOutput `json:"data"`
	mfaKEY   string
	Verifier generateLoginChallengeCodeVerifier
}

type authParamsInput struct {
	client        *retryablehttp.Client
	email         string
	password      string
	tokenName     string
	tokenValue    string
	authParamsURL string
	debug         bool
}

type AuthParamsOutput struct {
	Identifier    string `json:"identifier"`
	PasswordSalt  string `json:"pw_salt"`
	PasswordCost  int64  `json:"pw_cost"`
	PasswordNonce string `json:"pw_nonce"`
	Version       string `json:"version"`
	TokenName     string
	Verifier      generateLoginChallengeCodeVerifier
}

type requestRefreshTokenInput struct {
	url          string
	accessToken  string
	refreshToken string
	debug        bool
}

type RequestRefreshTokenOutput struct {
	AccessToken       string `json:"access_token"`
	RefreshToken      string `json:"refresh_token"`
	AccessExpiration  int64  `json:"access_expiration"`
	RefreshExpiration int64  `json:"refresh_expiration"`
	ReadOnlyAccess    int    `json:"read_only_access"`
}

func RequestRefreshToken(client *retryablehttp.Client, url, accessToken, refreshToken string, debug bool) (output RefreshSessionResponse, err error) {
	if client == nil {
		client = common.NewHTTPClient()
	}

	var reqBodyBytes []byte
	apiVer := common.APIVersion

	// Check if tokens are cookie-based (version 2) format: "2:privateIdentifier"
	accessParts := strings.Split(accessToken, ":")
	refreshParts := strings.Split(refreshToken, ":")
	isCookieBased := len(accessParts) >= 2 && len(refreshParts) >= 2 && accessParts[0] == "2" && refreshParts[0] == "2"

	if isCookieBased {
		// For cookie-based sessions, send empty body - authentication is via cookies
		reqBodyBytes = []byte(fmt.Sprintf(`{"api":"%s"}`, apiVer))
	} else {
		// For header-based sessions, send tokens in request body
		reqBodyBytes = []byte(fmt.Sprintf(`{"api":"%s","access_token":"%s","refresh_token":"%s"}`, apiVer, accessToken, refreshToken))
	}

	var refreshSessionReq *retryablehttp.Request

	log.DebugPrint(debug, fmt.Sprintf("refresh token url: %s with API version: %s", url, common.APIVersion), common.MaxDebugChars)

	refreshSessionReq, err = retryablehttp.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	refreshSessionReq.Header.Set(common.HeaderContentType, common.SNAPIContentType)
	refreshSessionReq.Header.Set("Connection", "keep-alive")

	// For cookie-based sessions, we need more context to set proper cookies
	// This function signature is limited - we need session context to extract actual cookie values
	// For now, we'll return an error indicating that cookie-based refresh requires a different approach
	if isCookieBased {
		err = fmt.Errorf("cookie-based session refresh requires session context - use RefreshSessionWithContext instead")
		return
	}

	var signInResp *http.Response

	start := time.Now()
	signInResp, err = client.Do(refreshSessionReq)
	elapsed := time.Since(start)

	log.DebugPrint(debug, fmt.Sprintf("refresh session | request took: %+v", elapsed), common.MaxDebugChars)

	if err != nil {
		return output, err
	}

	defer func() {
		_ = signInResp.Body.Close()
	}()

	var respBody []byte

	// readStart := time.Now()
	respBody, err = io.ReadAll(signInResp.Body)
	// logging.DebugPrint(input.debug, fmt.Sprintf("requestToken | response read took %+v", time.Since(readStart)))

	if err != nil {
		return
	}

	// unmarshal success
	var out RefreshSessionResponse

	err = json.Unmarshal(respBody, &out)
	if err != nil {
		return
	}

	return out, nil
}

func requestToken(input signInInput) (signInSuccess signInResponse, signInFailure ErrorResponse, err error) {
	var reqBodyBytes []byte

	e := url.PathEscape(input.email)

	var reqBody string

	apiVer := common.APIVersion

	if input.tokenName != "" {
		reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","%s":"%s","code_verifier":"%s"}`, apiVer, input.encPassword, e, input.tokenName, input.tokenValue, input.codeVerifier)
	} else {
		reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false,"hvm_token":""}`, apiVer, input.encPassword, e, input.codeVerifier)
	}
	log.DebugPrint(input.debug, fmt.Sprintf("sign-in request prepared with API version: %s", apiVer), common.MaxDebugChars)

	reqBodyBytes = []byte(reqBody)

	var signInURLReq *retryablehttp.Request

	log.DebugPrint(input.debug, fmt.Sprintf("sign-in url: %s", input.signInURL), common.MaxDebugChars)

	signInURLReq, err = retryablehttp.NewRequest(http.MethodPost, input.signInURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	signInURLReq.Header.Set(common.HeaderContentType, common.SNAPIContentType)
	signInURLReq.Header.Set("Connection", "keep-alive")

	var signInResp *http.Response

	start := time.Now()
	signInResp, err = input.client.Do(signInURLReq)
	elapsed := time.Since(start)

	log.DebugPrint(input.debug, fmt.Sprintf("requestToken | request took: %+v", elapsed), common.MaxDebugChars)

	if err != nil {
		return signInSuccess, signInFailure, err
	}

	defer func() {
		_ = signInResp.Body.Close()
	}()

	var signInRespBody []byte

	// readStart := time.Now()
	signInRespBody, err = io.ReadAll(signInResp.Body)
	// logging.DebugPrint(input.debug, fmt.Sprintf("requestToken | response read took %+v", time.Since(readStart)))

	if err != nil {
		return
	}
	// unmarshal success
	err = json.Unmarshal(signInRespBody, &signInSuccess)
	if err != nil {
		return
	}

	// Cookies are handled automatically by HTTP client cookie jar

	// unmarshal failure
	err = json.Unmarshal(signInRespBody, &signInFailure)
	if err != nil {
		return
	}

	return signInSuccess, signInFailure, err
}

func processDoAuthRequestResponse(response *http.Response, debug bool) (output doAuthRequestOutput, errResp ErrorResponse, err error) {
	var body []byte
	body, err = io.ReadAll(response.Body)
	if err != nil {
		return
	}

	return unmarshalAuthRequestResponse(response.StatusCode, body, debug)
}

func unmarshalAuthRequestResponse(statusCode int, body []byte, debug bool) (output doAuthRequestOutput, errResp ErrorResponse, err error) {
	switch statusCode {
	case http.StatusOK, http.StatusNotModified:
		err = json.Unmarshal(body, &output)
	case http.StatusNotFound, http.StatusBadRequest, http.StatusUnauthorized:
		err = json.Unmarshal(body, &errResp)
		if err == nil {
			log.DebugPrint(debug, fmt.Sprintf("status %d %+v", statusCode, errResp), common.MaxDebugChars)
			if statusCode == http.StatusUnauthorized {
				log.DebugPrint(debug, fmt.Sprintf("parsed %+v\n", errResp), common.MaxDebugChars)
			}
		}
	case http.StatusForbidden:
		err = fmt.Errorf("server returned 403 Forbidden response")
	default:
		err = fmt.Errorf("unhandled: %+v", statusCode)
	}

	return
}

// UnmarshalAuthRequestResponseForTest exposes unmarshalAuthRequestResponse for testing.
func UnmarshalAuthRequestResponseForTest(statusCode int, body []byte, debug bool) (doAuthRequestOutput, ErrorResponse, error) {
	return unmarshalAuthRequestResponse(statusCode, body, debug)
}

type ErrorResponseData struct {
	Error struct {
		Tag     string `json:"tag"`
		Message string `json:"message"`
		Payload struct {
			MFAKey string `json:"mfa_key"`
		}
	}
}

type ErrorResponse struct {
	Meta signInResponseMeta `json:"meta"`
	Data ErrorResponseData  `json:"data"`
}

// HTTP request bit.
func doAuthParamsRequest(input authParamsInput) (output doAuthRequestOutput, err error) {
	verifier := generateChallengeAndVerifierForLogin()

	var reqBodyBytes []byte
	var reqBody string

	apiVer := common.APIVersion

	if input.tokenName != "" {
		reqBody = fmt.Sprintf(`{"api":"%s","email":"%s","%s":"%s","code_challenge":"%s"}`, apiVer, input.email, input.tokenName, input.tokenValue, verifier.codeChallenge)
	} else {
		reqBody = fmt.Sprintf(`{"api":"%s","email":"%s","code_challenge":"%s"}`, apiVer, input.email, verifier.codeChallenge)
	}
	log.DebugPrint(input.debug, fmt.Sprintf("sign-in request prepared with API version: %s", apiVer), common.MaxDebugChars)
	reqBodyBytes = []byte(reqBody)

	var req *retryablehttp.Request

	req, err = retryablehttp.NewRequest(http.MethodPost, input.authParamsURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	req.Header.Set(common.HeaderContentType, common.SNAPIContentType)
	req.Header.Set("Connection", "keep-alive")

	var response *http.Response

	response, err = input.client.Do(req)
	if err != nil {
		return
	}

	defer func() {
		_ = response.Body.Close()
	}()

	var requestOutput doAuthRequestOutput

	var errResp ErrorResponse

	requestOutput, errResp, err = processDoAuthRequestResponse(response, input.debug)
	if err != nil {
		return
	}

	output.Data.Identifier = requestOutput.Data.Identifier
	output.Data.Version = requestOutput.Data.Version
	output.Data.PasswordCost = requestOutput.Data.PasswordCost
	output.Data.PasswordNonce = requestOutput.Data.PasswordNonce
	output.Data.PasswordSalt = requestOutput.Data.PasswordSalt
	output.mfaKEY = errResp.Data.Error.Payload.MFAKey
	output.Verifier = verifier

	return output, err
}

func getAuthParams(input authParamsInput) (output AuthParamsOutput, err error) {
	var authRequestOutput doAuthRequestOutput
	// if token name not provided, then make request without
	authRequestOutput, err = doAuthParamsRequest(input)
	if err != nil {
		return
	}

	output.Identifier = authRequestOutput.Data.Identifier
	output.PasswordNonce = authRequestOutput.Data.PasswordNonce
	output.Version = authRequestOutput.Data.Version
	output.TokenName = authRequestOutput.mfaKEY
	output.Verifier = authRequestOutput.Verifier

	return
}

type signInInput struct {
	client       *retryablehttp.Client
	email        string
	encPassword  string
	tokenName    string
	tokenValue   string
	signInURL    string
	debug        bool
	codeVerifier string
}

type KeyParams struct {
	Created     string `json:"created"`
	Identifier  string `json:"identifier"`
	Origination string `json:"origination"`
	PwNonce     string `json:"pw_nonce"`
	Version     string `json:"version"`
}

type User struct {
	UUID            string `json:"uuid"`
	Email           string `json:"email"`
	ProtocolVersion string `json:"protocolVersion"`
}

type SignInResponseDataSession struct {
	Debug            bool
	HTTPClient       *retryablehttp.Client
	SchemaValidation bool
	Server           string
	FilesServerUrl   string `json:"filesServerUrl"`
	Token            string
	MasterKey        string
	// ImporterItemsKeys is the key used to encrypt exported items and set during import only
	KeyParams         KeyParams `json:"keyParams"`
	AccessToken       string    `json:"access_token"`
	RefreshToken      string    `json:"refresh_token"`
	AccessExpiration  int64     `json:"access_expiration"`
	RefreshExpiration int64     `json:"refresh_expiration"`
	ReadOnlyAccess    bool      `json:"readonly_access"`
	PasswordNonce     string
}

type signInResponseData struct {
	Session   SignInResponseDataSession `json:"Session"`
	KeyParams KeyParams                 `json:"key_params"`
	User      User                      `json:"user"`
}

type signInResponseMeta struct {
	Auth   interface{} `json:"auth"`
	Server struct {
		FilesServerURL string `json:"filesServerUrl"`
	} `json:"server"`
}

type signInResponse struct {
	Meta signInResponseMeta `json:"meta"`
	Data signInResponseData `json:"data"`
}

type RefreshSessionResponse struct {
	Meta struct {
		Auth   interface{} `json:"auth"`
		Server struct {
			FilesServerURL string `json:"filesServerUrl"`
		} `json:"server"`
	} `json:"meta"`
	Data struct {
		Session struct {
			AccessToken       string `json:"access_token"`
			RefreshToken      string `json:"refresh_token"`
			AccessExpiration  int64  `json:"access_expiration"`
			RefreshExpiration int64  `json:"refresh_expiration"`
			ReadOnlyAccess    int    `json:"readonly_access"`
		} `json:"session"`
	} `json:"data"`
}

type registerResponse struct {
	User struct {
		UUID  string `json:"uuid"`
		Email string `json:"email"`
	}
	Token string `json:"token"`
}

type SignInInput struct {
	HTTPClient *retryablehttp.Client
	Email      string
	TokenName  string
	TokenVal   string
	Password   string
	APIServer  string
	Debug      bool
}

type SignInOutput struct {
	Session   SignInResponseDataSession
	KeyParams KeyParams
	User      User
	TokenName string
}

func processConnectionFailure(i error, reqURL string) error {
	switch {
	case strings.Contains(i.Error(), "no such host"):
		urlBits, pErr := url.Parse(reqURL)
		if pErr != nil {
			break
		}

		return fmt.Errorf("failed to connect to %s as %s cannot be resolved", reqURL, urlBits.Hostname())
	case strings.Contains(i.Error(), "StatusCode:503"):
		return fmt.Errorf("API server returned status 503: 'Service Unavailable'")
	case strings.Contains(i.Error(), "EOF"):
		return fmt.Errorf("API server returned an empty response")
	case strings.Contains(i.Error(), "unsupported protocol scheme"):
		if len(reqURL) > 0 {
			return fmt.Errorf("protocol is missing from API server URL: %s", reqURL)
		}

		return fmt.Errorf("API server URL is undefined")
	case strings.Contains(i.Error(), "i/o timeout"):
		return fmt.Errorf("failed to connect to %s within %d seconds", reqURL, common.ConnectionTimeout)
	case strings.Contains(i.Error(), "permission denied"):
		return fmt.Errorf("failed to connect to %s", reqURL)
	default:
		return fmt.Errorf("unhandled exception...\n- url: %s\n- error: %+v", reqURL, i.Error())
	}

	return i
}

// SignIn authenticates with the server using credentials and optional MFA
// in order to obtain the data required to interact with Standard Notes.
func SignIn(input SignInInput) (output SignInOutput, err error) {
	if input.APIServer == "" {
		input.APIServer = common.APIServer
	}

	if input.HTTPClient == nil {
		input.HTTPClient = common.NewHTTPClient()
	}

	output.Session.HTTPClient = input.HTTPClient

	getAuthParamsInput := authParamsInput{
		client:        input.HTTPClient,
		email:         input.Email,
		password:      input.Password,
		tokenValue:    input.TokenVal,
		tokenName:     input.TokenName,
		authParamsURL: input.APIServer + common.AuthParamsPath,
		debug:         input.Debug,
	}

	// request authentication parameters
	var getAuthParamsOutput AuthParamsOutput

	getAuthParamsOutput, err = getAuthParams(getAuthParamsInput)
	spew.Dump(getAuthParamsOutput)
	if err != nil {
		log.DebugPrint(input.Debug, fmt.Sprintf("getAuthParams error: %+v", err), common.MaxDebugChars)
		return output, processConnectionFailure(err, getAuthParamsInput.authParamsURL)
	}
	// fmt.Printf("getAuthParamsOutput: %#+v\n", getAuthParamsOutput)

	if getAuthParamsOutput.Version == "003" {
		err = fmt.Errorf("version 003 of Standard Notes is no longer supported")
		return
	}

	// if we received a token name then we need to request token value
	if getAuthParamsOutput.TokenName != "" {
		output.TokenName = getAuthParamsOutput.TokenName
		return
	}

	// generate encrypted password 004
	var genEncPasswordInput crypto.GenerateEncryptedPasswordInput

	genEncPasswordInput.UserPassword = input.Password
	genEncPasswordInput.Identifier = getAuthParamsOutput.Identifier
	genEncPasswordInput.PasswordNonce = getAuthParamsOutput.PasswordNonce
	genEncPasswordInput.Debug = input.Debug

	var _, sp string

	var mk string

	mk, sp, err = crypto.GenerateMasterKeyAndServerPassword004(genEncPasswordInput)
	if err != nil {
		return
	}

	// request token
	var tokenResp signInResponse

	var requestTokenFailure ErrorResponse
	tokenResp, requestTokenFailure, err = requestToken(signInInput{
		client:       input.HTTPClient,
		email:        input.Email,
		encPassword:  sp,
		tokenName:    input.TokenName,
		tokenValue:   input.TokenVal,
		signInURL:    input.APIServer + common.SignInPath,
		debug:        input.Debug,
		codeVerifier: getAuthParamsOutput.Verifier.codeVerifier,
	})

	if err != nil {
		log.DebugPrint(input.Debug, fmt.Sprintf("requestToken failure: %+v error: %+v", requestTokenFailure, err), common.MaxDebugChars)

		return
	}

	if requestTokenFailure.Data.Error.Message != "" {
		err = errors.New(strings.ToLower(requestTokenFailure.Data.Error.Message))
		return
	}

	// output.Session = tokenResp.Data.Session
	output.KeyParams = tokenResp.Data.KeyParams
	output.User = tokenResp.Data.User
	ds := SignInResponseDataSession{
		HTTPClient:        input.HTTPClient,
		MasterKey:         mk,
		KeyParams:         tokenResp.Data.KeyParams,
		AccessToken:       tokenResp.Data.Session.AccessToken,
		RefreshToken:      tokenResp.Data.Session.RefreshToken,
		AccessExpiration:  tokenResp.Data.Session.AccessExpiration,
		RefreshExpiration: tokenResp.Data.Session.RefreshExpiration,
		ReadOnlyAccess:    tokenResp.Data.Session.ReadOnlyAccess,
		PasswordNonce:     tokenResp.Data.KeyParams.PwNonce,
	}

	// Cookies are handled automatically by HTTP client cookie jar

	output.Session = ds

	// check if we need to add a post sign in delay
	psid, ok, envErr := common.ParseEnvInt64(common.EnvPostSignInDelay)
	if envErr != nil {
		panic(envErr)
	}
	if ok {
		log.DebugPrint(input.Debug, fmt.Sprintf("SignIn | sleeping %d milliseconds post sign in", psid), common.MaxDebugChars)
		time.Sleep(time.Duration(psid) * time.Millisecond)
	}

	return output, nil
}

// RequestRefreshTokenWithSession is a session-aware refresh function that handles both
// cookie-based (20240226) and header-based (20200115) authentication methods
func RequestRefreshTokenWithSession(session *SignInResponseDataSession, url string, debug bool) (output RefreshSessionResponse, err error) {
	if session.HTTPClient == nil {
		session.HTTPClient = common.NewHTTPClient()
	}

	var reqBodyBytes []byte
	apiVer := common.APIVersion

	// For both session types, the request body only contains the API version
	reqBodyBytes = []byte(fmt.Sprintf(`{"api":"%s"}`, apiVer))

	var refreshSessionReq *retryablehttp.Request

	log.DebugPrint(debug, fmt.Sprintf("refresh token url: %s with API version: %s", url, common.APIVersion), common.MaxDebugChars)

	refreshSessionReq, err = retryablehttp.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	refreshSessionReq.Header.Set(common.HeaderContentType, common.SNAPIContentType)
	refreshSessionReq.Header.Set("Connection", "keep-alive")

	// Use Authorization header - cookies are handled automatically by HTTP client cookie jar
	refreshSessionReq.Header.Set("Authorization", "Bearer "+session.AccessToken)
	log.DebugPrint(debug, "Using Authorization header for token refresh", common.MaxDebugChars)

	var signInResp *http.Response

	start := time.Now()
	signInResp, err = session.HTTPClient.Do(refreshSessionReq)
	elapsed := time.Since(start)

	log.DebugPrint(debug, fmt.Sprintf("refresh session | request took: %+v", elapsed), common.MaxDebugChars)

	if err != nil {
		return output, err
	}

	defer func() {
		_ = signInResp.Body.Close()
	}()

	var respBody []byte
	respBody, err = io.ReadAll(signInResp.Body)
	if err != nil {
		return
	}

	// unmarshal success
	err = json.Unmarshal(respBody, &output)
	if err != nil {
		return
	}

	// Cookies are handled automatically by HTTP client cookie jar

	return output, nil
}

type RefreshSessionInput struct {
	Email        string
	AccessToken  string
	RefreshToken string
	APIServer    string
	Debug        bool
}

type RefreshSessionOutput struct {
	Session   SignInResponseDataSession
	KeyParams KeyParams
	User      User
	TokenName string
}

type RegisterInput struct {
	Client      *retryablehttp.Client
	Password    string
	Email       string
	PWNonce     string
	Version     string
	Origination string
	Created     int64
	APIServer   string
	Debug       bool
}

func processDoRegisterRequestResponse(response *http.Response, debug bool) (token string, err error) {
	var body []byte

	body, err = io.ReadAll(response.Body)
	if err != nil {
		return
	}

	var errResp ErrorResponse
	_ = json.Unmarshal(body, &errResp)

	log.DebugPrint(debug, fmt.Sprintf("processDoRegisterRequestResponse | status code: %d error %s",
		response.StatusCode,
		errResp.Data.Error.Message), common.MaxDebugChars)

	switch response.StatusCode {
	case 200:
		var output registerResponse

		err = json.Unmarshal(body, &output)

		if err != nil {
			return
		}

		token = output.Token
	case 400:
		// unmarshal error response
		var errResp ErrorResponse

		err = json.Unmarshal(body, &errResp)
		if errResp.Data.Error.Message != "" {
			err = fmt.Errorf("email is already registered")
			return
		}
	case 404:
		log.DebugPrint(debug, fmt.Sprintf("status code: %d error %s", response.StatusCode, errResp.Data.Error.Message), common.MaxDebugChars)
		// email address not recognized
		err = fmt.Errorf("email address not recognized")
	case 401:
		// unmarshal error response
		var errResp ErrorResponse

		err = json.Unmarshal(body, &errResp)
		if errResp.Data.Error.Message != "" {
			err = fmt.Errorf("email is already registered")
			return
		}
	default:
		log.DebugPrint(debug, fmt.Sprintf("status code: %d error %s", response.StatusCode, errResp.Data.Error.Message), common.MaxDebugChars)
		err = fmt.Errorf("unhandled: %+v", response)

		return
	}

	return token, err
}

// Register creates a new user token
// Params: email, password, pw_cost, pw_nonce, version.
func (input RegisterInput) Register() (token string, err error) {
	if len(input.Password) < common.MinPasswordLength {
		err = fmt.Errorf("password must be at least %d characters", common.MinPasswordLength)

		return
	}

	if input.APIServer == "" {
		input.APIServer = common.APIServer
	}

	var pwNonce, serverPassword string
	_, pwNonce, _, serverPassword, err = generateInitialKeysAndAuthParamsForUser(input.Email, input.Password)
	if err != nil {
		return "", err
	}

	var req *retryablehttp.Request

	reqBody := fmt.Sprintf(`{"email":"%s","identifier":"%s","password":"%s","pw_nonce":"%s","version":"%s","origination":"registration","created":"1608473387799","api":"%s"}`, input.Email, input.Email, serverPassword, pwNonce, common.DefaultSNVersion, common.APIVersion)

	reqBodyBytes := []byte(reqBody)

	req, err = retryablehttp.NewRequest(http.MethodPost, input.APIServer+common.AuthRegisterPath, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	req.Header.Set(common.HeaderContentType, common.SNAPIContentType)
	req.Header.Set("Connection", "keep-alive")

	req.Host = input.APIServer

	var response *http.Response

	response, err = input.Client.Do(req)
	if err != nil {
		return
	}

	defer func() {
		_ = response.Body.Close()
	}()

	token, err = processDoRegisterRequestResponse(response, input.Debug)
	log.DebugPrint(true, token, common.MaxDebugChars)

	if err != nil {
		return
	}
	// TODO: Why create ItemsKey and Sync it?
	// create an ItemsKey and Sync it
	// sio, err := SignIn(SignInInput{
	// 	// _, err = SignIn(SignInInput{
	// 	Email:     input.Email,
	// 	Password:  input.Password,
	// 	APIServer: input.APIServer,
	// 	Debug:     input.Debug,
	// })
	// if err != nil {
	// 	return
	// }

	//
	// // create an ItemsKey and Sync it
	// ik, err := items.CreateItemsKey()
	// if err != nil {
	// 	return
	// }
	//
	// eKey, err := items.EncryptItemsKey(ik, &sio.Session, true)
	// if err != nil {
	// 	return
	// }
	//
	// so, err := items.Sync(items.SyncInput{
	// 	Session: &sio.Session,
	// 	Items:   []items.EncryptedItem{eKey},
	// })
	// if err != nil {
	// 	return
	// }
	//
	// if len(so.SavedItems) == 0 {
	// 	return "", fmt.Errorf("no items saved")
	// }

	return token, nil
}

func GenerateAuthData(ct, uuid string, kp KeyParams) string {
	if ct == common.SNItemTypeItemsKey {
		ad := struct {
			KP KeyParams `json:"kp"`
			U  string    `json:"u"`
			V  string    `json:"v"`
		}{
			KP: kp,
			U:  uuid,
			V:  kp.Version,
		}

		b, err := json.Marshal(ad)
		if err != nil {
			panic(err)
		}

		return string(b)
	}

	ad := struct {
		U string `json:"u"`
		V string `json:"v"`
	}{
		U: uuid,
		V: common.DefaultSNVersion,
	}

	b, err := json.Marshal(ad)
	if err != nil {
		panic(err)
	}

	return string(b)
}

func generateInitialKeysAndAuthParamsForUser(email, password string) (pw, pwNonce, masterKey, serverPassword string, err error) {
	var genInput crypto.GenerateEncryptedPasswordInput
	genInput.UserPassword = password
	// genInput.Version = defaultSNVersion
	genInput.Identifier = email
	// genInput.PasswordCost = common.DefaultPasswordCost

	// generate salt seed (password nonce)
	var src cryptoSource
	rnd := rand.New(src)

	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, 65)
	for i := range b {
		b[i] = letterRunes[rnd.Intn(len(letterRunes))]
	}

	genInput.PasswordNonce = string(b)
	pwNonce = string(b)[:32]
	// pw, _, _, err = generateEncryptedPasswordAndKeys(genInput)
	masterKey, serverPassword, err = crypto.GenerateMasterKeyAndServerPassword004(crypto.GenerateEncryptedPasswordInput{
		UserPassword:  password,
		Identifier:    email,
		PasswordNonce: pwNonce,
		Debug:         false,
	})

	return
}

// CliSignIn takes the server URL and credentials and sends them to the API to get a response including
// an authentication token plus the keys required to encrypt and decrypt SN items.
func CliSignIn(email, password, server string, debug bool) (session SignInResponseDataSession, err error) {
	httpClient := common.NewHTTPClient()
	sInput := SignInInput{
		HTTPClient: httpClient,
		Email:      email,
		Password:   password,
		APIServer:  server,
		Debug:      debug,
	}

	// attempt sign-in without MFA
	var sioNoMFA SignInOutput

	sioNoMFA, err = SignIn(sInput)
	if err != nil {
		return
	}

	// return Session if auth and master key returned
	if sioNoMFA.Session.AccessToken != "" && sioNoMFA.Session.RefreshExpiration != 0 {
		return sioNoMFA.Session, err
	}

	if sioNoMFA.TokenName != "" {
		// MFA token value required, so request
		var tokenValue string

		fmt.Print("token: ")

		_, err = fmt.Scanln(&tokenValue)
		if err != nil {
			return
		}
		// TODO: handle missing TokenName and Session
		// add token name and value to sign-in input
		sInput.TokenName = sioNoMFA.TokenName
		sInput.TokenVal = strings.TrimSpace(tokenValue)

		sOutTwo, sErrTwo := SignIn(sInput)
		if sErrTwo != nil {
			return session, sErrTwo
		}

		session = sOutTwo.Session
	}

	return session, nil
}

type generateLoginChallengeCodeVerifier struct {
	codeVerifier  string
	codeChallenge string
}

func generateChallengeAndVerifierForLogin() (loginCodeVerifier generateLoginChallengeCodeVerifier) {
	// generate salt seed (password nonce)
	var src cryptoSource
	rnd := rand.New(src)

	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, 65)
	for i := range b {
		b[i] = letterRunes[rnd.Intn(len(letterRunes))]
	}

	loginCodeVerifier.codeVerifier = string(b)[:64]
	sha25Hash := fmt.Sprintf("%x", sha256.Sum256([]byte(loginCodeVerifier.codeVerifier)))
	loginCodeVerifier.codeChallenge = string(base64.URLEncoding.EncodeToString([]byte(sha25Hash)))[:86]

	return loginCodeVerifier
}
