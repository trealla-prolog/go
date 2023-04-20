package trealla

import (
	_ "embed"

	"github.com/bytecodealliance/wasmtime-go/v8"
)

//go:embed libtpl.wasm
var tplWASM []byte

// type wasmFunc func(...any) (any, error)
type wasmFunc = *wasmtime.Func

var wasmEngine *wasmtime.Engine
var wasmModule *wasmtime.Module

func init() {
	cfg := wasmtime.NewConfig()
	cfg.SetStrategy(wasmtime.StrategyCranelift)
	cfg.SetCraneliftOptLevel(wasmtime.OptLevelSpeed)
	cfg.SetWasmBulkMemory(true)
	cfg.SetWasmMemory64(false)
	cfg.SetWasmSIMD(true)
	cfg.SetWasmMultiValue(true)
	cfg.SetWasmMultiMemory(true)
	// cfg.CacheConfigLoadDefault()
	wasmEngine = wasmtime.NewEngineWithConfig(cfg)

	var err error
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
