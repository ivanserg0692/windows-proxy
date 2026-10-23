package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// The live server remains owned by the root. This screen only renders snapshots.
type Running struct{}

func (running *Running) Resize(width, height int) {}

func (running *Running) Update(msg tea.Msg) (tea.Cmd, Intent) {
	if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "s" || key.String() == "S") {
		return nil, StopProxyIntent{}
	}
	return nil, nil
}

func (running *Running) View(snapshot Snapshot) Content {
	content := Content{Help: "S: stop   Esc: menu (proxy stays active)"}
	if snapshot.Running != nil {
		cfg, stats := snapshot.Running.Config, snapshot.Running.Stats
		content.Body = fmt.Sprintf("Windows:  %s\nWSL:      %s\nUpstream: %s:%d\n\nRequests: %d\nRetries:  %d\nFailures: %d", cfg.ListenEndpoint(), safe(cfg.Distribution), stats.Upstream, cfg.TargetPort, stats.Requests, stats.Retries, stats.Failures)
	}
	lines := snapshot.RecentEvents
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	if len(lines) > 0 {
		content.Footer = "\n\n" + muted.Render("Recent events\n"+strings.Join(lines, "\n"))
	}
	return content
}
