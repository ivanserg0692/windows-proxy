package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"wslproxy/internal/app"
	"wslproxy/internal/config"
)

func TestFormReportsInvalidPortWithoutSaving(t *testing.T) {
	logs := &Logs{}
	m := New(context.Background(), &app.Service{Store: config.Store{Path: filepath.Join(t.TempDir(), "config.json")}}, logs).(*model)
	m.config.Distribution = "Ubuntu"
	m.openForm()
	m.fields[1].SetValue("65536")
	if _, err := m.formConfig(); err == nil {
		t.Fatal("invalid port accepted")
	}
	m.fields[1].SetValue("8080")
	m.fields[4].SetValue("172.28.1.4")
	c, err := m.formConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.ManualTargetIP != "172.28.1.4" {
		t.Fatalf("manual IP missing: %+v", c)
	}
}

func TestLogsAreBoundedAndEscapeTerminalControls(t *testing.T) {
	logs := &Logs{}
	for i := 0; i < 20; i++ {
		logs.Logger().Error("failure\x1b[2J", "error", "bad\x00input")
	}
	lines := logs.Lines()
	if len(lines) != 8 {
		t.Fatalf("unbounded events: %d", len(lines))
	}
	if strings.ContainsAny(strings.Join(lines, ""), "\x1b\x00") {
		t.Fatal("terminal controls leaked")
	}
}

func TestSmallTerminalView(t *testing.T) {
	m := &model{width: 20, height: 10}
	view := m.View()
	if !view.AltScreen || !strings.Contains(view.Content, "Resize") {
		t.Fatalf("unexpected view: %+v", view)
	}
}
