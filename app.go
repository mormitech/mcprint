package main

import "context"

// App holds the methods the UI calls.
type App struct {
	ctx context.Context
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// Version is the app version from wails.json.
func (a *App) Version() string { return version() }
