package trealla

import (
	_ "embed"

	"github.com/wasmerio/wasmer-go/wasmer"
)

//go:embed libtpl.wasm
var tplWASM []byte

type wasmFunc func(...any) (any, error)

var wasmEngine = wasmer.NewEngine()
var wasmStore = wasmer.NewStore(wasmEngine)
var wasmModule *wasmer.Module

func init() {
	var err error
	// if wasmer.IsCompilerAvailable(wasmer.LLVM) {
	// 	wasmEngine = wasmer.NewEngineWithConfig(wasmer.NewConfig().UseLLVMCompiler())
	// }
	wasmModule, err = wasmer.NewModule(wasmStore, tplWASM)
	if err != nil {
		panic(err)
	}
}

var (
	// wasm_null  = wasmer.NewI32(0)
	wasm_false = wasmer.NewI32(0)
	wasm_true  = wasmer.NewI32(1)
)
