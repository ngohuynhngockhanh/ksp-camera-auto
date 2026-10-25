package bidahub

import "embed"

// StaticFS embeds the SCADA Dual-Canvas SPA and Shop Owner Portal assets.
//
//go:embed static/*
var StaticFS embed.FS
