const serverURL = "http://localhost:8080"

const fetchStateAndVerifyConsistency = async () => {
  const stateResponse = await fetch(serverURL + '/state');
  const serverState = await stateResponse.json();
  // TODO OGG: load local state from local storage
  const localState = serverState;
  console.log("curr state:", serverState)
  VerifyConsistency(localState, serverState, serverURL);
}

document.addEventListener("DOMContentLoaded", function() {
  (async function loadAndRunGoWasm() {
    const go = new Go();
    const response = await fetch("verifier.wasm");
    const buffer = await response.arrayBuffer();
    const result = await WebAssembly.instantiate(buffer, go.importObject);
    go.run(result.instance);

    fetchStateAndVerifyConsistency();
  })()
});