package main

import (
	"crypto/subtle"
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
)

var immudbClient = ImmudbClient{}

// BasicAuth middleware
func BasicAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(adminUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(adminPass)) != 1 {
			w.Header().Set(
				"WWW-Authenticate",
				`Basic realm="Please enter your username and password"`)
			w.WriteHeader(401)
			w.Write([]byte("You are Unauthorized to access the application.\n"))
			return
		}
		handler(w, r)
	}
}

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
	http.HandleFunc("/register-voter", BasicAuth(registerVoterHandler))
	http.HandleFunc("/vote", BasicAuth(voteHandler))
	http.HandleFunc("/voter-status", BasicAuth(getVoterStatusHandler))
	http.HandleFunc("/ballot", BasicAuth(getBallotHandler))
	fmt.Println("listening on port", port)

	// start server
	if err = http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Fatalf("error starting HTTP server: %v", err)
	}
}
