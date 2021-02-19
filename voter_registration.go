package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RegisterVoterRequest ...
type RegisterVoterRequest struct {
	IDNumber    string    `json:"id_number"`
	Name        string    `json:"name"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Address     string    `json:"address"`
	Email       string    `json:"email"`
}

func (p *RegisterVoterRequest) validate() error {
	var errs []string
	if len(p.IDNumber) == 0 {
		errs = append(errs, "id number is missing")
	}
	if len(p.Name) == 0 {
		errs = append(errs, "name is missing")
	}
	if p.DateOfBirth.IsZero() {
		errs = append(errs, "date of birth is missing")
	}
	if len(p.Address) == 0 {
		errs = append(errs, "address is missing")
	}
	if !isEmailValid(p.Email) {
		errs = append(errs, "email is invalid")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

// RegisterVoterResponse ...
type RegisterVoterResponse struct {
	RegistrationCode string `json:"registration_number"`
}

// Voter ...
type Voter struct {
	RegisterVoterRequest
	Approved time.Time `json:"approved"`
	Voted    time.Time `json:"voted"`
}

func registerVoterHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodPost) {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var payload RegisterVoterRequest
	err := decoder.Decode(&payload)
	if err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest,
			fmt.Sprintf("error parsing request body: %v", err))
		return
	}

	voterKey := []byte("immuvoting:voter:" + payload.IDNumber)
	if _, err := immudbClient.Get(voterKey, 0); err != nil && err == nil {
		writeErrorResponse(r, w, http.StatusTooManyRequests, "voter is already registered")
		return
	}

	voterBytes, err := json.Marshal(&Voter{RegisterVoterRequest: payload})
	if err != nil {
		writeErrorResponse(r, w, http.StatusUnprocessableEntity,
			fmt.Sprintf("error JSON-marshaling voter: %v", err))
		return
	}
	if err := immudbClient.Set(voterKey, voterBytes); err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError,
			fmt.Sprintf("error persisting registration: %v", err))
		return
	}

	registrationCode, err := uuid()
	if err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError,
			fmt.Sprintf("error generating registration code: %v", err))
		return
	}
	resPayload := RegisterVoterResponse{
		RegistrationCode: registrationCode,
	}

	writeJSONResponse(r, w, http.StatusOK, &resPayload)
}

func approveVoterHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodGet) {
		return
	}
	id := r.URL.Query().Get("id")
	if len(id) == 0 {
		writeErrorResponse(r, w, http.StatusBadRequest, "missing id query param")
		return
	}

	// TODO OGG: implement
}
