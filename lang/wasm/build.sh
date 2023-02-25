mkdir -p jsroot
set -e
cd ../../nexus
GOOS=js GOARCH=wasm go build -o libwbjs.wasm lib/libwbjs.go
cd -
cd jsroot
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" .
cp ../../../nexus/libwbjs.wasm .
cp ../index.html .
cd -
