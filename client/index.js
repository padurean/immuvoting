const serverURL = "http://localhost:8080"

const fetchStateAndVerifyConsistency = async () => {
  VerifyConsistency(serverURL);
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