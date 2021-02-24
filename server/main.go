package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
)

const (
	host      = "localhost"
	port      = "8080"
	adminUser = "admin"
	adminPass = "admin"

	// NikkiHaley candidate
	NikkiHaley = 1
	// KamalaHarris candidate
	KamalaHarris = 2
)

var immudbClient = ImmudbClient{}

func main() {
	fmt.Println(
		"    _                                       __  _\n" +
			"   (_)___ ___  ____ ___  __  ___   ______  / /_(_)___  ____ _\n" +
			"  / / __ `__ \\/ __ `__ \\/ / / / | / / __ \\/ __/ / __ \\/ __ `/\n" +
			" / / / / / / / / / / / / /_/ /| |/ / /_/ / /_/ / / / / /_/ /\n" +
			"/_/_/ /_/ /_/_/ /_/ /_/\\__,_/ |___/\\____/\\__/_/_/ /_/\\__, /\n" +
			"e l e c t i o n s  a n y o n e  c a n  v e r i f y  \\____/\n")
	// fmt.Print("e l e c t i o n s   a n y o n e   c a n   v e r i f y\n\n\n")

	// init immudb client
	immudbClient.Init(&ImmudbConfig{
		Address:       "localhost:3322",
		DB:            "defaultdb",
		User:          "immudb",
		Password:      "immudb",
		LocalStateDir: "",
	})
	if err := immudbClient.Connect(); err != nil {
		log.Fatalf("error connecting to immudb: %v", err)
	}
	defer immudbClient.Disconnect()

	// create immuvoting admin user
	hashedPassword, err := HashAndSaltPassword("admin")
	if err != nil {
		log.Fatalf("error hashing and salting password: %v", err)
	}
	if err := immudbClient.Set(
		[]byte("immuvoting:user:admin"), []byte(hashedPassword)); err != nil &&
		!errors.Is(err, ErrAlreadyExists) {
		log.Fatalf("error creating admin user: %v", err)
	}

	// setup HTTP handlers
	http.HandleFunc("/register-voter", cors(registerVoterHandler))
	http.HandleFunc("/vote", cors(voteHandler))
	http.HandleFunc("/voter-status", cors(getVoterStatusHandler))
	http.HandleFunc("/ballot", cors(getBallotHandler))
	http.HandleFunc("/state", cors(getStateHandler))
	http.HandleFunc("/results", cors(getResultsHandler))
	// NOTE: to add a handler which requires auth, wrap the handler with corsAndBasicAuth(...)
	fmt.Println("listening on port", port)

	// start server
	if err = http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Fatalf("error starting HTTP server: %v", err)
	}
}
