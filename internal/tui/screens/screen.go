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

type Content struct {
	Body   string
	Help   string
	Footer string
}
