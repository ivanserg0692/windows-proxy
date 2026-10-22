package wsl

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

type fakeRunner struct {
	calls    [][]string
	results  map[string][]byte
	failures map[string]error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	key := strings.Join(args, "|")
	return f.results[key], f.failures[key]
}

func utf16LE(text string, bom bool) []byte {
	var data []byte
	if bom {
		data = []byte{0xff, 0xfe}
	}
	for _, u := range utf16.Encode([]rune(text)) {
		data = binary.LittleEndian.AppendUint16(data, u)
	}
	return data
}

func TestDistributionEncoding(t *testing.T) {
	for _, data := range [][]byte{[]byte("Ubuntu\r\nDebian 日本語\r\n"), utf16LE("Ubuntu\r\nDebian 日本語\r\n", true), utf16LE("Ubuntu\r\nDebian 日本語\r\n", false)} {
		runner := &fakeRunner{results: map[string][]byte{"--list|--quiet": data}}
		names, err := (Client{Runner: runner}).Distributions(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(names, []string{"Ubuntu", "Debian 日本語"}) {
			t.Fatalf("wrong names: %v", names)
		}
	}
}

func TestDistributionErrors(t *testing.T) {
	runner := &fakeRunner{results: map[string][]byte{"--list|--quiet": nil}}
	client := Client{Runner: runner}
	names, err := client.Distributions(context.Background())
	if err != nil || len(names) != 0 {
		t.Fatalf("empty: %v %v", names, err)
	}
	if client.CheckDistribution(context.Background(), "Ubuntu") == nil {
		t.Fatal("missing distro accepted")
	}
	runner.failures = map[string]error{"--list|--quiet": fmt.Errorf("WSL unavailable")}
	if _, err := client.Distributions(context.Background()); err == nil {
		t.Fatal("runner error ignored")
	}
	if _, err := decodeOutput([]byte{0xff, 0xfe, 0x41}); err == nil {
		t.Fatal("invalid UTF-16 accepted")
	}
}

const addresses = `[
 {"ifname":"docker0","addr_info":[{"family":"inet","local":"172.17.0.1","scope":"global"}]},
 {"ifname":"eth0","addr_info":[{"family":"inet","local":"172.28.1.4","scope":"global"}]},
 {"ifname":"eth1","addr_info":[{"family":"inet","local":"10.0.0.4","scope":"global"}]},
 {"ifname":"lo","addr_info":[{"family":"inet","local":"127.0.0.1","scope":"host"}]}]`

func TestCandidatesUseValidatedRouteSource(t *testing.T) {
	runner := &fakeRunner{results: map[string][]byte{
		"--list|--quiet":                       []byte("Ubuntu"),
		"-d|Ubuntu|--|ip|-j|-4|address|show":   []byte(addresses),
		"-d|Ubuntu|--|ip|-4|route|get|1.1.1.1": []byte("1.1.1.1 via 172.28.1.1 dev eth1 src 10.0.0.4 uid 1000"),
	}}
	got, err := (Client{Runner: runner}).Candidates(context.Background(), "Ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].IP != netip.MustParseAddr("10.0.0.4") || !got[0].Preferred {
		t.Fatalf("wrong candidates: %+v", got)
	}
	if !reflect.DeepEqual(runner.calls[1], []string{"-d", "Ubuntu", "--", "ip", "-j", "-4", "address", "show"}) {
		t.Fatalf("wrong args: %v", runner.calls)
	}
}

func TestRouteFailureStillReturnsCandidates(t *testing.T) {
	runner := &fakeRunner{results: map[string][]byte{"--list|--quiet": []byte("Ubuntu"), "-d|Ubuntu|--|ip|-j|-4|address|show": []byte(addresses)}, failures: map[string]error{"-d|Ubuntu|--|ip|-4|route|get|1.1.1.1": fmt.Errorf("no route")}}
	got, err := (Client{Runner: runner}).Candidates(context.Background(), "Ubuntu")
	if err != nil || len(got) != 2 || got[0].Preferred || got[1].Preferred {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestUninstalledNameNeverExecuted(t *testing.T) {
	runner := &fakeRunner{results: map[string][]byte{"--list|--quiet": []byte("Ubuntu")}}
	_, err := (Client{Runner: runner}).Candidates(context.Background(), "Debian; echo unexpected")
	if err == nil || len(runner.calls) != 1 {
		t.Fatalf("unverified name executed: %v %v", runner.calls, err)
	}
}
