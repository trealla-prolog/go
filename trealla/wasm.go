package trealla

import (
	_ "embed"

	"github.com/wasmerio/wasmer-go/wasmer"
)

//go:embed tpl.wasm
var tplWASM []byte

var wasmEngine = wasmer.NewEngine()
var wasmStore = wasmer.NewStore(wasmEngine)
var wasmModule *wasmer.Module

func init() {
	var err error
	wasmModule, err = wasmer.NewModule(wasmStore, tplWASM)
	if err != nil {
		panic(err)
	}
}

type wasmFunc func(...any) (any, error)
