<img src="./client/immuvoting-logo.svg" height="85">

Electronic voting system allowing anyone to act as an auditor and verify that the election data has not been tampered.

_Powered by_ **[immudb](https://github.com/codenotary/immudb)**

---

## How to run it

### Prerequisites

- [immudb](https://github.com/codenotary/immudb) v0.9.x - the immutable database. GitHub repo is [here](https://github.com/codenotary/immudb). More details about it can be found on it's [official site](https://www.codenotary.com/technologies/immudb/).

- A modern browser (the web interface uses relatively new HTML and ES6 features - e.g. the `featch` API, `const` keyword etc.).

### Fire it up!

- Run **`immudb`**

**_NOTE_**: _**immuvoting**_ will try to connect to it using default config: `localhost`, port `3322`, database `defaultdb` and default credentials (have a look in _**immuvoting**_'s _server/main.go_ for more details)

- from _**immuvoting**_'s _server_ folder run:
   - `go get ./...`
   - `go run .` to start the HTTP API server (backend)

- a separate HTTP server needs to be started to serve the frontend (in the _client_ folder) - e.g. if using [VSCode](https://code.visualstudio.com), you can just use it's _**Go Live**_ feature; or you can use any other solution, like `python -m SimpleHTTPServer`.

**That's all.** You can now access the fronted at _localhost:&lt;xxx&gt;_

**_NOTE_**: Port number depends on the HTTP server you used: for [VSCode](https://code.visualstudio.com)'s _**Go Live**_ it's _**5500**_, for python's `SimpleHTTPServer` it's _**8000**_.

---

### Miscellanea

- The cryptographic verification of the election data (a.k.a. the _consistency proof_ or _tampering proof_) is written in [Go](https://golang.org) and resides in the _server/verifier.go_ folder. It is compiled to WASM and runs in the browser, on the voter's / auditor's machine, automatically at a fixed interval. For instructions on how to recompile it to WASM, see the [README](./server/verifier) in the _server/verifier_ folder.
