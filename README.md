<img src="./client/immuvoting-logo.svg" height="85">

Electronic voting system allowing anyone to act as an auditor and (cryptographically) verify that the election data has not been tampered.

_Powered by_ **[immudb](https://github.com/codenotary/immudb)**

---

## How to run it

### Prerequisites

- [immudb](https://github.com/codenotary/immudb) v0.9.x - the immutable database. GitHub repo is [here](https://github.com/codenotary/immudb). More details about it can be found on it's [official site](https://www.codenotary.com/technologies/immudb/).

- A modern browser (the web interface uses relatively new HTML and ES6 features - e.g. the `featch` API, `const` keyword etc.).

### Fire it up!

- Run **`immudb`**

**_NOTE_**: _**immuvoting**_ will try to connect to it using default config: `localhost`, port `3322`, database `defaultdb` and default credentials (have a look in [server/main.go](./server/main.go) for more details)

- from _**immuvoting**_'s [server](./server) folder run:
  - `go get ./...`
  - `go run .` to start the HTTP API server (backend)

- a separate HTTP server needs to be started to serve the frontend (in the [client](./client) folder) - e.g. if using [VSCode](https://code.visualstudio.com), you can just use it's _**Go Live**_ feature; or you can use any other solution, like `python -m SimpleHTTPServer`.

**That's all.** You can now access the fronted at [http://localhost:&lt;xxx&gt;](http://localhost:5500).

**_NOTE_**: Port number depends on the HTTP server you used: default port for [VSCode](https://code.visualstudio.com)'s _**Go Live**_ it's `5500`, for python's `SimpleHTTPServer` it's `8000`.

---

## Miscellanea

- The cryptographic verification of the election data (a.k.a. the _consistency proof_ or _tampering proof_) is written in [Go](https://golang.org) and it's code resides in [server/verifier/verifier.go](./server/verifier/verifier.go). It is compiled to [WebAssembly](https://webassembly.org) (i.e. to [client/verifier.wasm](./client/verifier.wasm)) and runs in the browser, on the voter's / auditor's machine, automatically at a fixed interval. For instructions on how to recompile it to WASM, see the [README](./server/verifier/README.md) in the [server/verifier](./server/verifier) folder.

### How it works: Consistency proofs and Merkle Trees

- The cryptographic verification, a.k.a the _consistency proof_, is achieved by leveraging the core features of [immudb](https://www.codenotary.com/technologies/immudb/). It is based on [Merkle Trees](https://brilliant.org/wiki/merkle-tree/). More details about this can be read, for example, in [this article](https://transparency.dev/verifiable-data-structures/) or in [this one](https://computersciencewiki.org/index.php/Merkle_proof) which explains the [Merkle proofs](https://computersciencewiki.org/index.php/Merkle_proof).
