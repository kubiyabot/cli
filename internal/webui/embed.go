package webui

import "embed"

// staticFS embeds the static web UI files
// These files are built from the webui/ directory and copied here
//
//go:embed static/*
var staticFS embed.FS
