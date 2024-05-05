package trealla

import (
	_ "embed"

	"github.com/bytecodealliance/wasmtime-go/v20"
)

//go:embed libtpl.wasm
var tplWASM []byte

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

	// libtpl.wasm is built with: -zstack-size=8388608
	cfg.SetMaxWasmStack(8388608)

	// cfg.CacheConfigLoadDefault()
	wasmEngine = wasmtime.NewEngineWithConfig(cfg)

	var err error
	wasmModule, err = wasmtime.NewModule(wasmEngine, tplWASM)
	if err != nil {
		panic(err)
	}
}

var (
	wasmFalse int32 = 0
	wasmTrue  int32 = 1
)

// type wint = int32

const (
	ptrSize = 4
	align   = 1
)
