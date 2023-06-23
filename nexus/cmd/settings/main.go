package main

import (
	"flag"
	"github.com/wandb/wandb/nexus/pkg/tools/settings"
)

func main() {
	jsonFilePath := flag.String("cfg", "config.json", "json file")
	tmplPath := flag.String("tmpl", "struct.go.tmpl", "template file")
	outFileName := flag.String("out", "settings.go", "output file")
	flag.Parse()

	generate.Generate(*jsonFilePath, *tmplPath, *outFileName)
}
