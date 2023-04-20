package trealla

import (
	_ "embed"

	"github.com/bytecodealliance/wasmtime-go/v8"
)

//go:embed libtpl.wasm
var tplWASM []byte

// type wasmFunc func(...any) (any, error)
type wasmFunc = *wasmtime.Func

// var wasmEngine = wasmer.NewEngine()
var wasmEngine *wasmtime.Engine

func init() {
	cfg := wasmtime.NewConfig()
	cfg.SetWasmBulkMemory(true)
	cfg.SetStrategy(wasmtime.StrategyCranelift)
	cfg.SetCraneliftOptLevel(wasmtime.OptLevelSpeed)
	wasmEngine = wasmtime.NewEngineWithConfig(cfg)
}

// var wasmStore = wasmtime.NewStore(wasmEngine)
var wasmModule *wasmtime.Module

func init() {
	var err error
	// if wasmer.IsCompilerAvailable(wasmer.LLVM) {
	// 	wasmEngine = wasmer.NewEngineWithConfig(wasmer.NewConfig().UseLLVMCompiler())
	// }
	wasmModule, err = wasmtime.NewModule(wasmEngine, tplWASM)
	if err != nil {
		panic(err)
	}
}

var (
	// wasm_null  = wasmer.NewI32(0)
	// wasmFalse = wasmer.NewI32(0)
	// wasmTrue  = wasmer.NewI32(1)

	wasmFalse int32 = 0
	wasmTrue  int32 = 1
)
