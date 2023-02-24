package main

import (
	"encoding/base64"
	"syscall/js"
	"github.com/wandb/wandb/nexus/server"
)

var globApiKey string

func main() {
	js.Global().Set("base64", encodeWrapper())
	js.Global().Set("wandb_login", wandb_login())
	js.Global().Set("wandb_init", wandb_init())
	<-make(chan bool)
}

func wandb_login() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) == 0 {
			return wrap("", "Not enough arguments")
		}
		apiKey := args[0].String()
		globApiKey = apiKey
		return wrap("junkdoit", "")
	})
}

func wandb_init() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		base_url := "https://api.wandb.ai"
		run_id := server.ShortID(8)
		settings := &server.Settings{
			BaseURL:  base_url,
			ApiKey:   globApiKey,
			SyncFile: "something.wandb",
			NoWrite: true,
			Offline:  false}
		_ = server.LibStartSettings(settings, run_id)
		return wrap("junkdoit", "")
	})
}

func encodeWrapper() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) == 0 {
			return wrap("", "Not enough arguments")
		}
		input := args[0].String()
		return wrap(base64.StdEncoding.EncodeToString([]byte(input)), "")
	})
}

func wrap(encoded string, err string) map[string]interface{} {
	return map[string]interface{}{
		"error":   err,
		"encoded": encoded,
	}
}
