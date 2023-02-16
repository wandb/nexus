package main

import (
	"log"
	"net/http"
)

const (
	AddSrv = ":8080"
	TemplatesDir = "jsroot/"
)

func main() {
	fileSrv := http.FileServer(http.Dir(TemplatesDir))
	if err := http.ListenAndServe(AddSrv, fileSrv); err != nil {
		log.Fatalln(err)
	}
}
