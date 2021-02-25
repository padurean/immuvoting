package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/codenotary/immudb/pkg/api/schema"
)

type middleware func(http.HandlerFunc) http.HandlerFunc

// builds the middleware chain recursively
func chain(handler http.HandlerFunc, m ...middleware) http.HandlerFunc {
	if len(m) == 0 {
		return handler
	}
	return m[0](chain(handler, m[1:]...))
}

// CORS middleware
func cors(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		handler(w, r)
	}
}

// basicAuth middleware
func basicAuth(handler http.HandlerFunc) http.HandlerFunc {
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

func corsAndBasicAuth(handler http.HandlerFunc) http.HandlerFunc {
	return chain(handler, cors, basicAuth)
}

// RegisterVoterRequest ...
type RegisterVoterRequest struct {
	CitizenID   string    `json:"citizen_id"`
	Name        string    `json:"name"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Address     string    `json:"address"`
	Email       string    `json:"email"`
}

func (req *RegisterVoterRequest) validate() error {
	var errs []string
	if len(req.CitizenID) == 0 {
		errs = append(errs, "citizen ID is missing")
	}
	if len(req.Name) == 0 {
		errs = append(errs, "name is missing")
	}
	if req.DateOfBirth.IsZero() {
		errs = append(errs, "date of birth is missing")
	}
	if len(req.Address) == 0 {
		errs = append(errs, "address is missing")
	}
	if !isEmailValid(req.Email) {
		errs = append(errs, "email is invalid")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

// Voter ...
type Voter struct {
	RegisterVoterRequest
	RegistrationApproved time.Time `json:"registration_approved"`
	Voted                time.Time `json:"voted"`
}

// RegisterVoterResponse ...
type RegisterVoterResponse struct {
	VoterID  string `json:"voter_id"`
	BallotID string `json:"ballot_id"`
}

func registerVoterHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodPost) {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var payload RegisterVoterRequest
	err := decoder.Decode(&payload)
	if err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest, nil,
			fmt.Sprintf("error parsing request body: %v", err))
		return
	}
	if err := payload.validate(); err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest, nil, err.Error())
		return
	}

	citizenKey := []byte("immuvoting:citizen:" + payload.CitizenID)
	if _, err := immudbClient.Get(citizenKey, 0); err == nil {
		writeErrorResponse(r, w, http.StatusTooManyRequests, err, "already registered")
		return
	}

	voterID, err := uuid()
	if err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error generating voter ID")
		return
	}
	voterKey := []byte("immuvoting:voter:" + voterID)
	voterBytes, err := json.Marshal(&Voter{
		RegisterVoterRequest: payload,
		RegistrationApproved: time.Now()})
	if err != nil {
		writeErrorResponse(r, w, http.StatusUnprocessableEntity, nil,
			fmt.Sprintf("error JSON-marshaling voter: %v", err))
		return
	}

	ballotID, err := uuid()
	if err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error generating ballot ID")
		return
	}
	ballotKey := []byte("immuvoting:ballot:" + ballotID)
	ballotValue := make([]byte, 2)
	binary.BigEndian.PutUint16(ballotValue, 0)

	if _, err := immudbClient.ExecAll(&schema.ExecAllRequest{
		Operations: []*schema.Op{
			{Operation: &schema.Op_Kv{Kv: &schema.KeyValue{Key: voterKey, Value: voterBytes}}},
			{Operation: &schema.Op_Ref{Ref: &schema.ReferenceRequest{Key: citizenKey, ReferencedKey: voterKey}}},
			{Operation: &schema.Op_Kv{Kv: &schema.KeyValue{Key: ballotKey, Value: ballotValue}}},
		},
	}); err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error persisting voter registration")
		return
	}

	resPayload := RegisterVoterResponse{
		VoterID:  voterID,
		BallotID: ballotID,
	}

	writeJSONResponse(r, w, http.StatusOK, &resPayload)
}

// VoteRequest ...
type VoteRequest struct {
	RegisterVoterResponse
	Vote uint16 `json:"vote"`
}

func (req *VoteRequest) validate() error {
	var errs []string
	if len(req.VoterID) == 0 {
		errs = append(errs, "voter ID is missing")
	}
	if len(req.BallotID) == 0 {
		errs = append(errs, "ballot ID is missing")
	}
	if req.Vote == 0 {
		errs = append(errs, "vote is missing")
	} else if req.Vote != KamalaHarris && req.Vote != NikkiHaley {
		errs = append(errs, "invalid vote")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func voteHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodPost) {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var payload VoteRequest
	err := decoder.Decode(&payload)
	if err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest, nil,
			fmt.Sprintf("error parsing request body: %v", err))
		return
	}
	if err := payload.validate(); err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest, nil, err.Error())
		return
	}

	voterKey := []byte("immuvoting:voter:" + payload.VoterID)
	voterBytes, err := immudbClient.Get(voterKey, 0)
	if err != nil {
		writeErrorResponse(r, w, http.StatusNotFound, err,
			"voter has never been registered")
		return
	}
	var voter Voter
	if err := json.Unmarshal(voterBytes, &voter); err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error JSON-unmarshaling persisted voter")
		return
	}
	if voter.RegistrationApproved.IsZero() {
		writeErrorResponse(r, w, http.StatusForbidden, nil,
			"voter registration has never been approved")
		return
	}
	if !voter.Voted.IsZero() {
		writeErrorResponse(r, w, http.StatusForbidden, nil,
			"voter has already voted")
		return
	}

	ballotKey := []byte("immuvoting:ballot:" + payload.BallotID)
	ballotBytes, err := immudbClient.Get(ballotKey, 0)
	if err != nil {
		writeErrorResponse(r, w, http.StatusNotFound, err, "no such ballot")
		return
	}
	existingVote := binary.BigEndian.Uint16(ballotBytes)
	if existingVote > 0 {
		writeErrorResponse(r, w, http.StatusForbidden, nil,
			"ballot has been already cast before")
		return
	}

	voter.Voted = time.Now()
	voterBytes, err = json.Marshal(&voter)
	if err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error JSON-marshaling voter before persisting it")
		return
	}

	ballotValue := make([]byte, 2)
	binary.BigEndian.PutUint16(ballotValue, payload.Vote)

	if _, err := immudbClient.ExecAll(&schema.ExecAllRequest{
		Operations: []*schema.Op{
			{Operation: &schema.Op_Kv{Kv: &schema.KeyValue{Key: voterKey, Value: voterBytes}}},
			{Operation: &schema.Op_Kv{Kv: &schema.KeyValue{Key: ballotKey, Value: ballotValue}}},
		},
	}); err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error persisting updated voter and ballot")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetVoterStatusResponse ...
type GetVoterStatusResponse struct {
	RegistrationApproved time.Time `json:"approved"`
	Voted                time.Time `json:"voted"`
}

func getVoterStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodGet) {
		return
	}

	voterID := r.URL.Query().Get("voter_id")
	if len(voterID) == 0 {
		writeErrorResponse(r, w, http.StatusBadRequest, nil,
			"voter_id query param is missing")
		return
	}

	voterKey := []byte("immuvoting:voter:" + voterID)
	voterBytes, err := immudbClient.Get(voterKey, 0)
	if err != nil {
		writeErrorResponse(r, w, http.StatusNotFound, err,
			"voter has never been registered")
		return
	}
	var voter Voter
	if err := json.Unmarshal(voterBytes, &voter); err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error JSON-unmarshaling persisted voter")
		return
	}

	resPayload := GetVoterStatusResponse{
		RegistrationApproved: voter.RegistrationApproved,
		Voted:                voter.Voted,
	}

	writeJSONResponse(r, w, http.StatusOK, &resPayload)
}

// GetBallotResponse ...
type GetBallotResponse struct {
	Vote uint16 `json:"vote"`
}

func getBallotHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodGet) {
		return
	}

	ballotID := r.URL.Query().Get("ballot_id")
	if len(ballotID) == 0 {
		writeErrorResponse(r, w, http.StatusBadRequest, nil,
			"ballot_id query param is missing")
		return
	}

	ballotKey := []byte("immuvoting:ballot:" + ballotID)
	ballotBytes, err := immudbClient.Get(ballotKey, 0)
	if err != nil {
		writeErrorResponse(r, w, http.StatusNotFound, err, "no such ballot")
		return
	}

	resPayload := GetBallotResponse{Vote: binary.BigEndian.Uint16(ballotBytes)}

	writeJSONResponse(r, w, http.StatusOK, &resPayload)
}

// GetStateResponse ...
type GetStateResponse struct {
	TXID   uint64 `json:"tx_id"`
	TXHash string `json:"tx_hash"`
}

func getStateHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodGet) {
		return
	}
	state, err := immudbClient.CurrentState()
	if err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error fetching current state")
		return
	}
	res := GetStateResponse{TXID: state.TxId, TXHash: base64.StdEncoding.EncodeToString(state.TxHash)}
	writeJSONResponse(r, w, http.StatusOK, &res)
}

func getVerifiableTransactionHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodGet) {
		return
	}

	serverTXStr := r.URL.Query().Get("server_tx")
	if len(serverTXStr) == 0 {
		writeErrorResponse(r, w, http.StatusBadRequest, nil,
			"server_tx query param is missing")
		return
	}
	serverTX, err := strconv.ParseUint(serverTXStr, 10, 64)
	if err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest, err,
			"server_tx query param is not an unisigned int")
		return
	}

	localTXStr := r.URL.Query().Get("local_tx")
	if len(serverTXStr) == 0 {
		writeErrorResponse(r, w, http.StatusBadRequest, nil,
			"local_tx query param is missing")
		return
	}
	localTX, err := strconv.ParseUint(localTXStr, 10, 64)
	if err != nil {
		writeErrorResponse(r, w, http.StatusBadRequest, err,
			"local_tx query param is not an unisigned int")
		return
	}

	verifiableTX, err := immudbClient.VerifiableTXByID(serverTX, localTX)
	if err != nil {
		writeErrorResponse(r, w, http.StatusInternalServerError, err,
			"error fetching verifiable transaction")
		return
	}
	writeJSONResponse(r, w, http.StatusOK, verifiableTX)
}

func getResultsHandler(w http.ResponseWriter, r *http.Request) {
	if !isHTTPMethodValid(r, w, http.MethodGet) {
		return
	}
	// TODO OGG: implement
}
