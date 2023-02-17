package main

import (
	"encoding/base64"
	"syscall/js"
)

func main() {
	js.Global().Set("base64", encodeWrapper())
	js.Global().Set("wandb_init", wandb_init())
	<-make(chan bool)
}

func wandb_init() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return wrap("junk", "")
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
