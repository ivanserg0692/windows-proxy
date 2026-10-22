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
	uiModel := New(context.Background(), &app.Service{Store: config.Store{Path: filepath.Join(t.TempDir(), "config.json")}}, logs).(*model)
	uiModel.config.Distribution = "Ubuntu"
	uiModel.openForm()
	uiModel.fields[1].SetValue("65536")
	if _, err := uiModel.formConfig(); err == nil {
		t.Fatal("invalid port accepted")
	}
	uiModel.fields[1].SetValue("8080")
	uiModel.fields[4].SetValue("172.28.1.4")
	cfg, err := uiModel.formConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManualTargetIP != "172.28.1.4" {
		t.Fatalf("manual IP missing: %+v", cfg)
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
	uiModel := &model{width: 20, height: 10}
	view := uiModel.View()
	if !view.AltScreen || !strings.Contains(view.Content, "Resize") {
		t.Fatalf("unexpected view: %+v", view)
	}
}
