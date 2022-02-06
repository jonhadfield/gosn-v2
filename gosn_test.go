package gosn

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	testSession      *Session
	testUserEmail    string
	testUserPassword string
)

func localTestMain() {
	localServer := "http://ramea:3000"
	testUserEmail = fmt.Sprintf("ramea-%s", strconv.FormatInt(time.Now().UnixNano(), 16))
	testUserPassword = "secretsanta"

	rInput := RegisterInput{
		Password:  testUserPassword,
		Email:     testUserEmail,
		APIServer: localServer,
		Version:   defaultSNVersion,
		Debug:     true,
	}

	_, err := rInput.Register()
	if err != nil {
		panic(fmt.Sprintf("failed to register with: %s", localServer))
	}

	signIn(localServer, testUserEmail, testUserPassword)
}

func signIn(server, email, password string) {
	ts, err := CliSignIn(email, password, server, true)
	if err != nil {
		log.Fatal(err)
	}

	debugPrint(true, fmt.Sprintf("logged in as %s", email))

	testSession = &ts
}

func TestMain(m *testing.M) {
	if os.Getenv("SN_SERVER") == "" || strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		localTestMain()
	} else {
		signIn(os.Getenv("SN_SERVER"), os.Getenv("SN_EMAIL"), os.Getenv("SN_PASSWORD"))
	}

	if strings.ToLower(os.Getenv("SN_DEBUG")) == "true" {
		testSession.Debug = true
	}

	if _, err := Sync(SyncInput{Session: testSession}); err != nil {
		log.Fatal(err)
	}

	os.Exit(m.Run())
}
