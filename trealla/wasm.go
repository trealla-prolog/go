package trealla

import (
	"context"
	_ "embed"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed libtpl.wasm
var tplWASM []byte

type wasmFunc = api.Function

var wasmEngine wazero.Runtime
var wasmModule wazero.CompiledModule

func init() {
	ctx := context.Background()
	wasmEngine = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, wasmEngine)

	_, err := wasmEngine.NewHostModuleBuilder("trealla").
		NewFunctionBuilder().WithFunc(hostCall).Export("host-call").
		NewFunctionBuilder().WithFunc(hostResume).Export("host-resume").
		Instantiate(ctx)
	if err != nil {
		panic(err)
	}

	wasmModule, err = wasmEngine.CompileModule(ctx, tplWASM)
	if err != nil {
		panic(err)
	}
}

var (
	wasmFalse uint32 = 0
	wasmTrue  uint32 = 1
)

const (
	ptrSize  = 4
	align    = 1
	pageSize = 64 * 1024
)
