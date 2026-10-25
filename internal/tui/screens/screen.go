package screens

import (
	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
)

// Snapshot exposes values for presentation, without access to the root,
// application services, or the live server.
type Snapshot struct {
	Config       config.Config
	ProxyActive  bool
	Running      *RunningSnapshot
	RecentEvents []string
}

type RunningSnapshot struct {
	Config config.Config
	Stats  proxy.Snapshot
}

// Content contains the screen body, bounded by its allocated area.
type Content struct {
	Body string
}
