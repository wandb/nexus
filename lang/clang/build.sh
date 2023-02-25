set -e
cd ../../nexus
go build -buildmode=c-archive lib/libwbnexus.go
cd -
# gcc -pthread foo.c foo.a -o foo
# gcc -pthread foo.c foo.a -framework Cocoa -framework IOKit -framework Security  -o foo
gcc -pthread train.c libwb.c ../../nexus/libwbnexus.a -framework Cocoa -framework Security -o train
