package screens

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"wslproxy/internal/app"
	"wslproxy/internal/config"
	"wslproxy/internal/proxy"
	"wslproxy/internal/wsl"
)

func TestFormsOwnIndependentDraftsAndReturnSaveRequests(t *testing.T) {
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	first, second := &Form{}, &Form{}
	first.Open(cfg)
	second.Open(cfg)
	first.fields[0].SetValue("127.0.0.2")
	first.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd, intent := first.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	request, ok := intent.(SaveConfigIntent)
	if !ok || cmd != nil || request.Config.ListenAddress != "127.0.0.2" {
		t.Fatalf("expected synchronous save request, got %T", intent)
	}
	if second.fields[0].Value() != cfg.ListenAddress || second.focus != 0 || cfg.ListenAddress == "127.0.0.2" {
		t.Fatal("editing one form affected another form or its input configuration")
	}
}

func TestFormReportsInvalidPortWithoutSaving(t *testing.T) {
	form := &Form{}
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	form.Open(cfg)
	form.fields[1].SetValue("65536")
	if _, err := form.configuration(); err == nil {
		t.Fatal("invalid port accepted")
	}
	form.fields[1].SetValue("8080")
	form.fields[4].SetValue("172.28.1.4")
	cfg, err := form.configuration()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManualTargetIP != "172.28.1.4" {
		t.Fatalf("manual IP missing: %+v", cfg)
	}
}

func TestDiagnosticScreenRendersLimitedSafeReport(t *testing.T) {
	diagnostics := &Diagnostics{}
	report := app.Report{Selected: netip.MustParseAddr("172.28.1.4")}
	for index := 0; index < 5; index++ {
		report.Probes = append(report.Probes, app.Probe{
			Candidate: wsl.Candidate{IP: report.Selected, Interface: "eth0\x1b[2J"},
			Duration:  time.Millisecond, Error: "failure\x00",
		})
	}
	diagnostics.SetReport(report)
	content := diagnostics.View(Snapshot{Config: config.Default()})
	if strings.Count(content.Body, "FAIL:") != 4 || !strings.Contains(content.Body, "More addresses found") || !strings.Contains(content.Body, "Selected: 172.28.1.4") {
		t.Fatalf("unexpected report: %q", content.Body)
	}
	if strings.ContainsAny(content.Body, "\x1b\x00") {
		t.Fatal("terminal controls leaked from the report")
	}
}

func TestRunningScreenUsesValueSnapshotAndLastThreeEvents(t *testing.T) {
	running := &Running{}
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	content := running.View(Snapshot{
		Running:      &RunningSnapshot{Config: cfg, Stats: proxy.Snapshot{Upstream: "172.28.1.4", Requests: 12, Retries: 3, Failures: 2}},
		RecentEvents: []string{"oldest event", "second event", "third event", "last event"},
	})
	for _, expected := range []string{"Ubuntu", "172.28.1.4", "Requests: 12", "Retries:  3", "Failures: 2"} {
		if !strings.Contains(content.Body, expected) {
			t.Fatalf("status is missing %q: %q", expected, content.Body)
		}
	}
	if strings.Contains(content.Footer, "oldest event") {
		t.Fatalf("unexpected recent events: %q", content.Footer)
	}
	previous := -1
	for _, event := range []string{"second event", "third event", "last event"} {
		position := strings.Index(content.Footer, event)
		if position <= previous {
			t.Fatalf("missing or out-of-order event %q: %q", event, content.Footer)
		}
		previous = position
	}
}

func TestFormDistributionSelectionRetainsFocusAndDraft(t *testing.T) {
	cfg := config.Default()
	cfg.Distribution = "Ubuntu"
	form := &Form{}
	form.Open(cfg)
	form.fields[0].SetValue("127.0.0.2")
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd, intent := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	choose, ok := intent.(ChooseDistributionIntent)
	if !ok || !choose.ForForm || cmd != nil {
		t.Fatal("distribution field did not request synchronous selection for the form")
	}
	form.SelectDistribution("Debian")
	if form.focus != 2 || !form.fields[2].Focused() || form.fields[0].Value() != "127.0.0.2" || form.fields[2].Value() != "Debian" {
		t.Fatal("distribution selection lost the draft or focused field")
	}
}

func TestDistributionScreensOwnSelectionAndReturnItsPurpose(t *testing.T) {
	formSelector, menuSelector := NewDistributions(), NewDistributions()
	formSelector.SetNames([]string{"Ubuntu", "Debian"}, 80, 30)
	menuSelector.SetNames([]string{"Ubuntu", "Debian"}, 80, 30)
	formSelector.Open(true)
	menuSelector.Open(false)
	formSelector.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	for _, tc := range []struct {
		screen  *Distributions
		name    string
		forForm bool
	}{{formSelector, "Debian", true}, {menuSelector, "Ubuntu", false}} {
		cmd, intent := tc.screen.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		selected, ok := intent.(DistributionSelectedIntent)
		if !ok || cmd != nil || selected.Name != tc.name || selected.ForForm != tc.forForm {
			t.Fatalf("selection state or purpose leaked between screens: %+v", intent)
		}
	}
}
