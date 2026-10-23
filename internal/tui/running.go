package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// The live server remains owned by the root. This screen only renders snapshots.
type runningScreen struct{}

func (running *runningScreen) resize(width, height int) {}

func (running *runningScreen) update(msg tea.Msg) (tea.Cmd, screenIntent) {
	if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "s" || key.String() == "S") {
		return nil, stopProxyIntent{}
	}
	return nil, nil
}

func (running *runningScreen) view(snapshot screenSnapshot) screenContent {
	content := screenContent{help: "S: stop   Esc: menu (proxy stays active)"}
	if snapshot.running != nil {
		cfg, stats := snapshot.running.config, snapshot.running.stats
		content.body = fmt.Sprintf("Windows:  %s\nWSL:      %s\nUpstream: %s:%d\n\nRequests: %d\nRetries:  %d\nFailures: %d", cfg.ListenEndpoint(), safe(cfg.Distribution), stats.Upstream, cfg.TargetPort, stats.Requests, stats.Retries, stats.Failures)
	}
	lines := snapshot.recentEvents
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	if len(lines) > 0 {
		content.footer = "\n\n" + muted.Render("Recent events\n"+strings.Join(lines, "\n"))
	}
	return content
}
