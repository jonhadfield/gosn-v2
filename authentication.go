package gosn

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type cryptoSource struct{}

func (s cryptoSource) Seed(seed int64) {}

func (s cryptoSource) Int63() int64 {
	return int64(s.Uint64() & ^uint64(1<<63))
}

func (s cryptoSource) Uint64() (v uint64) {
	err := binary.Read(crand.Reader, binary.BigEndian, &v)
	if err != nil {
		log.Fatal(err)
	}

	return v
}

type doAuthRequestOutput struct {
	Data   authParamsOutput `json:"data"`
	mfaKEY string
}

type authParamsInput struct {
	email         string
	password      string
	tokenName     string
	tokenValue    string
	authParamsURL string
	debug         bool
}

type authParamsOutput struct {
	Identifier    string `json:"identifier"`
	PasswordSalt  string `json:"pw_salt"`
	PasswordCost  int64  `json:"pw_cost"`
	PasswordNonce string `json:"pw_nonce"`
	Version       string `json:"version"`
	TokenName     string
}

func requestToken(input signInInput) (signInSuccess signInResponse, signInFailure errorResponse, err error) {
	var reqBodyBytes []byte

	e := url.PathEscape(input.email)

	var reqBody string

	apiVer := "20200115"

	if input.tokenName != "" {
		reqBody = `{"api":"` + apiVer + `","password":"` + input.encPassword + `","email":"` + e + `","` + input.tokenName + `":"` + input.tokenValue + `"}`
	} else {
		reqBody = `{"api":"` + apiVer + `","password":"` + input.encPassword + `","email":"` + e + `"}`
	}

	reqBodyBytes = []byte(reqBody)

	var signInURLReq *http.Request

	debugPrint(input.debug, fmt.Sprintf("sign-in url: %s", input.signInURL))

	signInURLReq, err = http.NewRequest(http.MethodPost, input.signInURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	signInURLReq.Header.Set("content-Type", "application/json")
	signInURLReq.Header.Set("Connection", "keep-alive")

	var signInResp *http.Response

	start := time.Now()
	signInResp, err = httpClient.Do(signInURLReq)
	elapsed := time.Since(start)

	debugPrint(input.debug, fmt.Sprintf("requestToken | request took: %+v", elapsed))

	if err != nil {
		return signInSuccess, signInFailure, err
	}

	defer func() {
		_ = signInResp.Body.Close()
	}()

	var signInRespBody []byte

	// readStart := time.Now()
	signInRespBody, err = ioutil.ReadAll(signInResp.Body)
	// debugPrint(input.debug, fmt.Sprintf("requestToken | response read took %+v", time.Since(readStart)))

	if err != nil {
		return
	}
	// unmarshal success
	err = json.Unmarshal(signInRespBody, &signInSuccess)
	if err != nil {
		return
	}

	// unmarshal failure
	err = json.Unmarshal(signInRespBody, &signInFailure)
	if err != nil {
		return
	}

	return signInSuccess, signInFailure, err
}

func processDoAuthRequestResponse(response *http.Response, debug bool) (output doAuthRequestOutput, errResp errorResponse, err error) {
	var body []byte
	body, err = ioutil.ReadAll(response.Body)
	switch response.StatusCode {
	case 200:
		err = json.Unmarshal(body, &output)
		if err != nil {
			return
		}
	case 304:
		err = json.Unmarshal(body, &output)
		if err != nil {
			return
		}
	case 404:
		// email address not recognized
		err = json.Unmarshal(body, &errResp)
		if err != nil {
			return
		}

		debugPrint(debug, fmt.Sprintf("status 404 %+v", errResp))
		return
	case 400:
		// most likely authentication missing or SN API has changed
		err = json.Unmarshal(body, &errResp)
		if err != nil {
			return
		}

		debugPrint(debug, fmt.Sprintf("status 400 %+v", errResp))
	case 401:
		// need mfa token
		// unmarshal error response
		err = json.Unmarshal(body, &errResp)
		if err != nil {
			return
		}

		debugPrint(debug, fmt.Sprintf("status 401 %+v", errResp))
	case 403:
		// server has denied request
		// unmarshal error response
		err = fmt.Errorf("server returned 403 Forbidden response")
		return
	default:
		err = fmt.Errorf("unhandled: %+v", response)
		return
	}

	return
}

type errorResponseData struct {
	Error struct {
		Tag     string `json:"tag"`
		Message string `json:"message"`
		Payload struct {
			MFAKey string `json:"mfa_key"`
		}
	}
}

type errorResponse struct {
	Meta signInResponseMeta `json:"meta"`
	Data errorResponseData  `json:"data"`
}

// HTTP request bit.
func doAuthParamsRequest(input authParamsInput) (output doAuthRequestOutput, err error) {
	// make initial params request without mfa token
	var reqURL string
	e := url.QueryEscape(input.email)
	if input.tokenName == "" {
		// initial request
		reqURL = input.authParamsURL + "?email=" + e + "&api=20200115"
	} else {
		// request with mfa
		reqURL = input.authParamsURL + "?email=" + e + "&" + input.tokenName + "=" + input.tokenValue
	}
	var req *http.Request

	req, err = http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return
	}

	var response *http.Response

	response, err = httpClient.Do(req)
	if err != nil {
		return
	}

	defer func() {
		_ = response.Body.Close()
	}()

	var requestOutput doAuthRequestOutput

	var errResp errorResponse

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

	return output, err
}

func getAuthParams(input authParamsInput) (output authParamsOutput, err error) {
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

	return
}

type generateEncryptedPasswordInput struct {
	userPassword string
	authParamsOutput
	debug bool
}

type signInInput struct {
	email       string
	encPassword string
	tokenName   string
	tokenValue  string
	signInURL   string
	debug       bool
}

type KeyParams struct {
	Created     string `json:"created"`
	Identifier  string `json:"identifier"`
	Origination string `json:"origination"`
	PwNonce     string `json:"pw_nonce"`
	Version     string `json:"version"`
}

type user struct {
	UUID  string `json:"uuid"`
	Email string `json:"email"`
}

type signInResponseData struct {
	Session   Session   `json:"Session"`
	KeyParams KeyParams `json:"key_params"`
	User      user      `json:"user"`
}

type signInResponseMeta struct {
	Auth interface{} `json:"auth"`
}

type signInResponse struct {
	Meta signInResponseMeta `json:"meta"`
	Data signInResponseData `json:"data"`
}

type registerResponse struct {
	User struct {
		UUID  string `json:"uuid"`
		Email string `json:"email"`
	}
	Token string `json:"token"`
}

type SignInInput struct {
	Email     string
	TokenName string
	TokenVal  string
	Password  string
	APIServer string
	Debug     bool
}

type SignInOutput struct {
	Session   Session
	KeyParams KeyParams
	User      user
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
		return fmt.Errorf("failed to connect to %s within %d seconds", reqURL, connectionTimeout)
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
		input.APIServer = apiServer
	}

	getAuthParamsInput := authParamsInput{
		email:         input.Email,
		password:      input.Password,
		tokenValue:    input.TokenVal,
		tokenName:     input.TokenName,
		authParamsURL: input.APIServer + authParamsPath,
		debug:         input.Debug,
	}

	// request authentication parameters
	var getAuthParamsOutput authParamsOutput

	getAuthParamsOutput, err = getAuthParams(getAuthParamsInput)
	if err != nil {
		debugPrint(input.Debug, fmt.Sprintf("getAuthParams error: %+v", err))
		err = processConnectionFailure(err, getAuthParamsInput.authParamsURL)

		return
	}

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
	var genEncPasswordInput generateEncryptedPasswordInput

	genEncPasswordInput.userPassword = input.Password
	genEncPasswordInput.Identifier = getAuthParamsOutput.Identifier
	genEncPasswordInput.TokenName = input.TokenName
	genEncPasswordInput.PasswordNonce = getAuthParamsOutput.PasswordNonce
	genEncPasswordInput.Version = getAuthParamsOutput.Version
	genEncPasswordInput.debug = input.Debug

	var _, sp string

	var mk string

	mk, sp, err = generateMasterKeyAndServerPassword004(genEncPasswordInput)
	if err != nil {
		return
	}

	// request token
	var tokenResp signInResponse

	var requestTokenFailure errorResponse
	tokenResp, requestTokenFailure, err = requestToken(signInInput{
		email:       input.Email,
		encPassword: sp,
		tokenName:   input.TokenName,
		tokenValue:  input.TokenVal,
		signInURL:   input.APIServer + signInPath,
		debug:       input.Debug,
	})

	if err != nil {
		debugPrint(input.Debug, fmt.Sprintf("requestToken failure: %+v error: %+v", requestTokenFailure, err))

		return
	}

	if requestTokenFailure.Data.Error.Message != "" {
		err = fmt.Errorf(strings.ToLower(requestTokenFailure.Data.Error.Message))
		return
	}

	output.Session = tokenResp.Data.Session
	output.KeyParams = tokenResp.Data.KeyParams
	output.User = tokenResp.Data.User
	output.Session.MasterKey = mk
	output.Session.KeyParams = tokenResp.Data.KeyParams
	output.Session.Debug = input.Debug
	output.Session.Token = tokenResp.Data.Session.Token
	output.Session.Server = input.APIServer

	output.Session.PasswordNonce = getAuthParamsOutput.PasswordNonce

	// check if we need to add a post sign in delay
	pside := os.Getenv("SN_POST_SIGN_IN_DELAY")
	if pside != "" {
		psid, psidErr := strconv.ParseInt(pside, 10, 64)
		if psidErr != nil {
			panic(fmt.Sprintf("failed to parse SN_POST_SIGN_IN_DELAY value as int64: %v", pside))
		}

		debugPrint(input.Debug, fmt.Sprintf("SignIn | sleeping %d milliseconds post sign in", psid))
		time.Sleep(time.Duration(psid) * time.Millisecond)
	}

	return output, err
}

type RegisterInput struct {
	Password    string
	Email       string
	Identifier  string
	PWNonce     string
	Version     string
	Origination string
	Created     int64
	// API         string
	APIServer string
	Debug     bool
}

func processDoRegisterRequestResponse(response *http.Response, debug bool) (token string, err error) {
	var body []byte

	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	var errResp errorResponse
	_ = json.Unmarshal(body, &errResp)

	debugPrint(debug, fmt.Sprintf("processDoRegisterRequestResponse | status code: %d error %s",
		response.StatusCode,
		errResp.Data.Error.Message))

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
		var errResp errorResponse

		err = json.Unmarshal(body, &errResp)
		if errResp.Data.Error.Message != "" {
			err = fmt.Errorf("email is already registered")
			return
		}
	case 404:
		debugPrint(debug, fmt.Sprintf("status code: %d error %s", response.StatusCode, errResp.Data.Error.Message))
		// email address not recognized
		err = fmt.Errorf("email address not recognized")
	case 401:
		// unmarshal error response
		var errResp errorResponse

		err = json.Unmarshal(body, &errResp)
		if errResp.Data.Error.Message != "" {
			err = fmt.Errorf("email is already registered")
			return
		}
	default:
		debugPrint(debug, fmt.Sprintf("status code: %d error %s", response.StatusCode, errResp.Data.Error.Message))
		err = fmt.Errorf("unhandled: %+v", response)

		return
	}

	return token, err
}

// Register creates a new user token
// Params: email, password, pw_cost, pw_nonce, version.
func (input RegisterInput) Register() (token string, err error) {
	if input.APIServer == "" {
		input.APIServer = apiServer
	}
	var pwNonce, serverPassword string
	_, pwNonce, _, serverPassword, err = generateInitialKeysAndAuthParamsForUser(input.Email, input.Password)

	var req *http.Request

	reqBody := `{"email":"` + input.Email + `","identifier":"` + input.Email + `","password":"` + serverPassword + `","pw_nonce":"` + pwNonce + `","version":"` + defaultSNVersion + `","origination":"registration","created":"1608473387799","api":"20200115"}`

	reqBodyBytes := []byte(reqBody)

	req, err = http.NewRequest(http.MethodPost, input.APIServer+authRegisterPath, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return
	}

	req.Header.Set("content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")

	req.Host = input.APIServer

	var response *http.Response

	response, err = httpClient.Do(req)
	if err != nil {
		return
	}

	defer func() {
		_ = response.Body.Close()
	}()

	token, err = processDoRegisterRequestResponse(response, input.Debug)
	if err != nil {
		return
	}

	// create an ItemsKey and Sync it
	sio, err := SignIn(SignInInput{
		//_, err = SignIn(SignInInput{
		Email:     input.Email,
		Password:  input.Password,
		APIServer: input.APIServer,
		Debug:     input.Debug,
	})
	if err != nil {
		return
	}

	// create an ItemsKey and Sync it
	ik, err := sio.Session.CreateItemsKey()

	eKey, err := ik.Encrypt(&sio.Session, true)
	if err != nil {
		return
	}

	so, err := Sync(SyncInput{
		Session: &sio.Session,
		Items:   []EncryptedItem{eKey},
	})
	if err != nil {
		return
	}

	if len(so.SavedItems) == 0 {
		return "", fmt.Errorf("no items saved")
	}

	return token, err
}

func (Session) CreateItemsKey() (ItemsKey, error) {
	ik := NewItemsKey()
	// creating an items key is done during registration or when exporting, in which case it will always be default
	// ik.Default = true
	// ik.Content.Default = true
	ik.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	ik.CreatedAt = time.Now().UTC().Format(timeLayout)

	return ik, nil
}

func generateInitialKeysAndAuthParamsForUser(email, password string) (pw, pwNonce, masterKey, serverPassword string, err error) {
	var genInput generateEncryptedPasswordInput
	genInput.userPassword = password
	genInput.Version = defaultSNVersion
	genInput.Identifier = email
	genInput.PasswordCost = defaultPasswordCost

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
	masterKey, serverPassword, err = generateMasterKeyAndServerPassword004(generateEncryptedPasswordInput{
		userPassword: password,
		authParamsOutput: authParamsOutput{
			Identifier:    email,
			PasswordNonce: pwNonce,
			Version:       "",
			TokenName:     "",
		},
	})

	return
}

// CliSignIn takes the server URL and credentials and sends them to the API to get a response including
// an authentication token plus the keys required to encrypt and decrypt SN items.
func CliSignIn(email, password, apiServer string, debug bool) (session Session, err error) {
	sInput := SignInInput{
		Email:     email,
		Password:  password,
		APIServer: apiServer,
		Debug:     debug,
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

	return session, err
}

func (s *Session) Valid() bool {
	if s == nil {
		return false
	}

	switch {
	case s.RefreshToken == "":
		debugPrint(s.Debug, "session is missing refresh token")
		return false
	case s.AccessToken == "":
		debugPrint(s.Debug, "session is missing access token")
		return false
	case s.MasterKey == "":
		debugPrint(s.Debug, "session is missing master key")
		return false
	case s.AccessExpiration == 0:
		debugPrint(s.Debug, "Access Expiration is 0")
		return false
	case s.RefreshExpiration == 0:
		debugPrint(s.Debug, "Refresh Expiration is 0")
		return false
	}

	// check no duplicate item keys
	seen := make(map[string]int)
	for x := range s.ItemsKeys {
		if seen[s.ItemsKeys[x].UUID] > 0 {
			return false
		}

		seen[s.ItemsKeys[x].UUID]++
	}

	return true
}
