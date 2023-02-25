mkdir -p bin
go build -o bin/runweb runweb.go
echo http://localhost:8080/
./bin/runweb
