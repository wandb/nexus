cd ../../nexus
go build -buildmode=c-shared -o libwbnexus.dylib lib/libwbnexus.go
cd -
cp ../../nexus/libwbnexus.dylib .
