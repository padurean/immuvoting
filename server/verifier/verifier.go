package main

import (
	"encoding/base64"
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

// VerifyConsistency ...
func VerifyConsistency(this js.Value, args []js.Value) interface{} {
	localTXID := uint64(args[0].Get("tx_id").Int())
	localTXHash, err := base64.StdEncoding.DecodeString(args[0].Get("tx_hash").String())
	if err != nil {
		println("error Base64-decoding local TX hash: %s", err.Error())
		return nil
	}

	serverTXID := uint64(args[1].Get("tx_id").Int())
	serverTXHash, err := base64.StdEncoding.DecodeString(args[1].Get("tx_hash").String())
	if err != nil {
		println("error Base64-decoding server TX hash: %s", err.Error())
		return nil
	}

	serverURL := args[2].String()
	vTXURL := fmt.Sprintf(
		"%s/verifiable-tx?server_tx=%d&local_tx=%d", serverURL, serverTXID, localTXID)

	go func() {
		client := http.Client{Timeout: 5 * time.Second}
		req, err := http.NewRequest(http.MethodGet, vTXURL, nil)
		if err != nil {
			println(
				"error creating new HTTP GET %s request to fetch verifiable TX: %s", vTXURL, err.Error())
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			println("error fetching verifiable TX with HTTP GET %s: %s", vTXURL, err.Error())
			return
		}
		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			println("error reading bytes from the HTTP GET %s response: %s", vTXURL, err.Error())
			return
		}
		var vTX schema.VerifiableTx
		if err := json.Unmarshal(bodyBytes, &vTX); err != nil {
			println("error JSON-unmarshaling VerifiableTX from HTTP GET %s response bytes %s: %s",
				vTXURL, string(bodyBytes), err.Error())
			return
		}
		proof := schema.DualProofFrom(vTX.DualProof)
		localHash := schema.DigestFrom(localTXHash)
		serverHash := schema.DigestFrom(serverTXHash)
		verified := store.VerifyDualProof(proof, localTXID, serverTXID, localHash, serverHash)
		println(
			"local (tx=", localTXID, ", hash=", fmt.Sprintf("%s", localHash), "), ",
			"server (tx=", serverTXID, ", hash=", fmt.Sprintf("%s", serverHash), "), ",
			"verified:", verified)
	}()

	return nil
}
