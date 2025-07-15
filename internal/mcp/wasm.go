package mcp

import (
	"context"
	"embed"
	"io/fs"
	"sync"

	"github.com/extism/go-sdk"
	"golang.org/x/sync/semaphore"
)

//go:embed wfdsl.wasm
var wfDslWasmData embed.FS

var pluginPool sync.Pool

var sem = semaphore.NewWeighted(10)

func newWFDslWasmPlugin() error {
	path := "wfdsl.wasm"

	data, err := fs.ReadFile(wfDslWasmData, path)
	if err != nil {
		return err
	}

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{Data: data},
		},
	}
	pluginConfig := extism.PluginConfig{EnableWasi: true}

	pluginPool.New = func() interface{} {
		ctx := context.Background()
		plugin, err := extism.NewPlugin(ctx, manifest, pluginConfig, []extism.HostFunction{})
		if err != nil {
			return nil
		}
		return &struct {
			plugin *extism.Plugin
			ctx    context.Context
		}{plugin: plugin, ctx: ctx}
	}

	return nil
}
