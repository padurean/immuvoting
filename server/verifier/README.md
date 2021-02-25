# Client-side consistency verifier

## 1. Build as WASM and copy to _client_ folder

From the _server_ folder (one level up) run:

```console
GOOS=js GOARCH=wasm go build -o ../client/verifier.wasm ./verifier/verifier.go
```

## 2. Make sure _wasm_exec.js_ is present in the _client_ folder

Copy _wasm_exec.js_ from your Go distribution:

```console
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./client/
```
