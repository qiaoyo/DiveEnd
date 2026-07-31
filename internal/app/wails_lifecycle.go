package app

import (
	"context"
	"net/http"
)

// Startup adapts the internal lifecycle hook to Wails' callback.
func Startup(a *App, ctx context.Context) {
	a.startup(ctx)
}

// BeforeClose adapts the internal close guard to Wails' callback.
func BeforeClose(a *App, ctx context.Context) bool {
	return a.beforeClose(ctx)
}

// Shutdown adapts the internal shutdown hook to Wails' callback.
func Shutdown(a *App, ctx context.Context) {
	a.shutdown(ctx)
}

// AssetServerHandler exposes the app-owned PDF asset handler to the root
// desktop entrypoint without adding a Wails-bound App method.
func AssetServerHandler(a *App) http.Handler {
	return a.assetServerHandler()
}
