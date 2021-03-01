package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"syscall/js"
	"time"

	"github.com/codenotary/immudb/embedded/store"
	"github.com/codenotary/immudb/pkg/api/schema"
)

func main() {
	c := make(chan struct{}, 0)
	println("Go WebAssembly initialized")
	js.Global().Set("VerifyConsistency", js.FuncOf(VerifyConsistency))
	<-c
}

// State ...
type State struct {
	TXID   uint64 `json:"tx_id"`
	TXHash []byte `json:"tx_hash"`
}

// Equals ...
func (s *State) Equals(ss *State) bool {
	if s == nil || ss == nil {
		return false
	}
	return s.TXID == ss.TXID && bytes.Compare(s.TXHash, ss.TXHash) == 0
}

// VerifyConsistency ...
func VerifyConsistency(this js.Value, args []js.Value) interface{} {
	serverURL := args[0].String()

	go func() {
		client := http.Client{Timeout: 5 * time.Second}

		// get local state
		var localState *State
		localStateJS := js.Global().Get("localStorage").Call("getItem", "immuvotingState")
		if !js.Null().Equal(localStateJS) {
			localStateStr := localStateJS.String()
			println("local state:", localStateStr)
			localState = &State{}
			if err := json.Unmarshal([]byte(localStateStr), localState); err != nil {
				println("error JSON-unmarshaling local state", localStateStr, ": ", err.Error())
				return
			}
		}

		// get server state
		stateURL := serverURL + "/state"
		req, err := http.NewRequest(http.MethodGet, stateURL, nil)
		if err != nil {
			println(
				"error creating new HTTP GET %s request to fetch server state:",
				stateURL, err.Error())
			return
		}
		var serverState State
		if _, err := httpDo(&client, req, &serverState, "server state:"); err != nil {
			println(err.Error())
			return
		}

		var verified bool
		if localState != nil {
			// get verifiable transaction
			vTXURL := fmt.Sprintf(
				"%s/verifiable-tx?server_tx=%d&local_tx=%d",
				serverURL, serverState.TXID, localState.TXID)
			req, err = http.NewRequest(http.MethodGet, vTXURL, nil)
			if err != nil {
				println(
					"error creating new HTTP GET", vTXURL,
					"request to fetch verifiable TX:", err.Error())
				return
			}
			var vTX schema.VerifiableTx
			if httpStatus, err := httpDo(&client, req, &vTX, ""); err != nil {
				errMsg := err.Error()
				if httpStatus == http.StatusNotFound {
					errMsg = fmt.Sprintf(
						"verification error: one of the 2 tx IDs was not found on server: %v", err)
				}
				println(errMsg)
				return
			}

			// do the verification
			proof := schema.DualProofFrom(vTX.DualProof)
			localTXHash := schema.DigestFrom(localState.TXHash)
			serverTXHash := schema.DigestFrom(serverState.TXHash)
			verified = store.VerifyDualProof(
				proof, localState.TXID, serverState.TXID, localTXHash, serverTXHash)
			println("verified:", verified)

			now := time.Now().Format(time.RFC3339)
			resultDOMElem := js.Global().Get("document").Call("getElementById", "tampering-result")
			if !verified {
				resultDOMElem.Set("innerHTML", "<span class=\"audit-failed\">Tampered!</span> @ "+now)
			} else {
				resultDOMElem.Set("innerHTML", "<span class=\"audit-ok\">OK</span> @ "+now)
			}
		}

		// override the local state with the fresh server state
		if (verified && !localState.Equals(&serverState)) || localState == nil {
			serverStateBs, err := json.Marshal(&serverState)
			if err != nil {
				println(
					"error JSON-marshaling server state", fmt.Sprintf("%+v", serverState),
					"before perisiting it to local storage:", err.Error())
				return
			}
			js.Global().Get("localStorage").Call("setItem", "immuvotingState", string(serverStateBs))
		}

	}()

	return nil
}

func httpDo(client *http.Client, req *http.Request, out interface{}, printRawPayload string) (int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error executing HTTP request %s: %s", req.URL, err)
	}
	defer resp.Body.Close()
	httpStatus := resp.StatusCode
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return httpStatus, fmt.Errorf(
			"error reading bytes from %s response: %s", req.URL, err)
	}
	if httpStatus < 200 || httpStatus > 299 {
		return httpStatus, fmt.Errorf(
			"%s responded with non-200 range code %d", req.URL, httpStatus)
	}
	if len(printRawPayload) > 0 {
		print(printRawPayload, fmt.Sprintf("%s", bodyBytes))
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return httpStatus, fmt.Errorf(
			"error JSON-unmarshaling response bytes \"%s\" from %s: %s",
			bodyBytes, req.URL, err)
	}
	return httpStatus, nil
}
