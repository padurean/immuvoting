package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
)

// NoOpWriter ...
type NoOpWriter struct{}

// Write ...
func (nw *NoOpWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

var emailRegex = regexp.MustCompile(
	"^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

func isEmailValid(e string) bool {
	if len(e) < 3 && len(e) > 254 {
		return false
	}
	return emailRegex.MatchString(e)
}

func isHTTPMethodValid(
	r *http.Request,
	w http.ResponseWriter,
	method string) bool {

	if r.Method != method {
		errMsg := fmt.Sprintf(
			"%s http method is not supported on %s resource", r.Method, r.URL.Path)
		writeErrorResponse(r, w, http.StatusMethodNotAllowed, errMsg)
		return false
	}
	return true
}

func writeJSONResponse(
	r *http.Request,
	w http.ResponseWriter,
	statusCode int,
	body interface{}) {

	log.Print(fmt.Sprintf(
		"%s %s - writing SUCCESS http %d response with json data: %+v",
		r.Method,
		r.URL.Path,
		statusCode,
		body))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(body)
}

func writeErrorResponse(
	r *http.Request,
	w http.ResponseWriter,
	statusCode int,
	errMsg string) {

	log.Print(fmt.Sprintf(
		"%s %s - writing ERROR http %d response with message: %s",
		r.Method,
		r.URL.Path,
		statusCode, errMsg))
	w.WriteHeader(statusCode)
	w.Write([]byte(http.StatusText(statusCode) + ": " + errMsg))
}

func uuid() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
