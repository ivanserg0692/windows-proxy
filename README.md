# WSL Proxy

Windows AMD64 HTTP reverse proxy with a terminal menu, WSL distribution selection,
TCP diagnostics and an unattended `serve` mode. The executable runs on Windows
without Go or Docker. Docker is required only for compilation and tests.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Build and run](#build-and-run)
- [Terminal interface](#terminal-interface)
- [Configuration and unattended mode](#configuration-and-unattended-mode)
- [Discovery and proxy behavior](#discovery-and-proxy-behavior)
- [Tests and troubleshooting](#tests-and-troubleshooting)
- [Code organization](#code-organization)

<!-- END doctoc -->

## Build and run

Requirements for building: Docker Desktop with the **Linux engine**. When this
project is stored in WSL, also enable Docker Desktop integration for that Ubuntu
distribution. The build uses the official `golang:1.27.3` image; CGO is disabled
for the Windows executable. All commands below are relative to this project's root.

Download dependencies once, in **WSL Ubuntu**, from the project directory:

```bash
docker run --rm \
  --mount "type=bind,source=$PWD,target=/src" \
  -w /src -v wslproxy-go-mod:/go/pkg/mod \
  golang:1.27.3 go mod download all
```

If you store the project on a Windows drive, this equivalent command works in
PowerShell from that directory:

```powershell
docker run --rm --mount "type=bind,source=$($PWD.Path),target=/src" -w /src -v wslproxy-go-mod:/go/pkg/mod golang:1.27.3 go mod download all
```

For a `\\wsl.localhost\Ubuntu\...` or `\\wsl$\Ubuntu\...` project, use the WSL
download command. PowerShell's provider-prefixed UNC path is not a Docker bind
source. Both workflows must use the same Docker engine and dependency volume.

In Windows PowerShell:

```powershell
.\build.cmd
.\bin\proxy.exe
```

In CMD use `build.cmd`. The script resolves its own directory, so it also works
when invoked from another working directory. For WSL shares it invokes Docker
through `wsl.exe` in the distribution hosting the source files. For local Windows
directories it invokes Docker directly. Other UNC shares and directories containing
commas are not supported by this bind-mount workflow.

Output: `bin/proxy.exe`. No Dockerfile, Compose services or runtime container is
needed. Existing web server, cluster and firewall configuration is not changed.
The module cache is `wslproxy-go-mod`; compiler cache is `wslproxy-go-build`.
Compilation uses `-mod=readonly` and `GOPROXY=off`: it does not silently fetch
missing dependencies. Re-run the download command if a cache is removed.

## Terminal interface

Open `proxy.exe` in Windows Terminal or PowerShell. The interface uses ordinary
Unicode borders and text, without icon fonts. Use arrows and Enter in menus.

1. Select **Select WSL distribution**. Enter saves the choice with current settings.
2. Open **Configure proxy**. Set the Windows IPv4 listen address and port, WSL
   target port, optional manual IPv4, connect timeout, retry count and delay.
   Tab/Shift+Tab or Up/Down changes fields. Enter on the distribution field opens
   the distribution list. **Ctrl+S** validates and saves; Esc discards form edits.
3. Open **Connection diagnostics** to inspect Windows-to-WSL TCP probes. R repeats
   diagnostics. No HTTP request is sent by this check.
4. Choose **Start proxy**. Saved settings are reloaded; WSL is resolved and tested
   before opening the Windows listener. The status screen shows actual counters.
5. **S** gracefully stops the server and releases the port. **Esc** returns to the
   menu while the proxy keeps running. **Exit** or Ctrl+C stops the application.

Configuration changes affect the next proxy start. During an operation Ctrl+C
cancels and exits. A forced stop follows a five-second graceful-shutdown limit.
Keep the terminal at least 40 columns by 20 rows; a larger window shows more events.
Errors are displayed inside the TUI; HTTP server logs do not write over the screen.

## Configuration and unattended mode

Configuration is stored at `%APPDATA%\WSLProxy\config.json` via `os.UserConfigDir`:

```json
{
  "listen_address": "0.0.0.0",
  "listen_port": 8080,
  "wsl_distribution": "Ubuntu",
  "target_port": 8001,
  "connect_timeout_ms": 2000,
  "connect_retries": 1,
  "retry_delay_ms": 100
}
```

Optional `"manual_target_ip": "172.28.1.4"` explicitly overrides discovery. Leave
it empty or omit it for automatic detection. Automatically discovered addresses
are never saved. The selected distribution must remain installed, including in
manual mode. No secrets are needed or stored.

Ports must be 1–65535; listen addresses are IPv4 literals, including `0.0.0.0`.
Connect timeout is 100–60000 ms, retries 0–10 and retry delay 0–10000 ms. Unknown or
duplicate JSON fields, invalid values, trailing objects and files over 64 KiB are
rejected. Saving uses a flushed temporary file in the same directory and a Windows
`MoveFileExW` replacement. A failed write does not truncate the previous config.
Concurrent writers use last-successful-write-wins semantics.

After saving through the TUI, run without a menu:

```powershell
.\bin\proxy.exe serve
```

This mode writes structured JSON logs to stderr, reports startup errors with a
nonzero exit code and handles Ctrl+C. It does not install a Windows service.
`proxy.exe --help` lists the modes. Statistics are logged every ten seconds.

## Discovery and proxy behavior

`internal/wsl` executes `wsl.exe --list --quiet`, decodes UTF-16LE/UTF-8 output,
and validates the selected name against installed distributions. Commands use
separate `os/exec` arguments without shell interpolation; each command has a
ten-second deadline and bounded captured output. Executing a Linux command can
start a stopped distribution. The application does not shut it down afterwards.

Discovery combines `ip -j -4 address show` and `ip -4 route get 1.1.1.1` inside
the selected distribution. It verifies the route's source IP and interface against
assigned global IPv4 addresses, excluding known container bridges and loopback.
The route query does not send an application request to 1.1.1.1. An unavailable
route does not prevent other assigned addresses from being checked.

Windows probes each candidate once on the configured target port. A reachable
validated route source is preferred; otherwise exactly one reachable candidate
is selected. Multiple reachable candidates without route evidence require a manual
IP. The program never chooses the first address from `hostname -I`.

Mirrored networking may require an explicit `127.0.0.1` target. Loopback is never
automatically selected: an open port could belong to Windows or another service.
TCP reachability does not prove the identity or health of an HTTP application.
Obvious self-proxy address/port combinations are rejected; indirect forwarding
loops through other proxies remain the operator's responsibility.

The proxy forwards HTTP/1.1 to a fixed configured upstream, preserves the incoming
Host, path and query, and replaces untrusted forwarding headers using
`ReverseProxy.Rewrite` and `SetXForwarded`. System HTTP proxy environment variables
are not used. CONNECT, protocol upgrades, HTTPS termination and arbitrary TCP/UDP
tunnels are not implemented.

`internal/proxy` uses `httputil.ReverseProxy` and standard-library HTTP connections.
A bounded pool of up to 32 `http.ClientConn` instances preserves keep-alive without
the request replay behavior of `http.Transport.RoundTrip`. Each request calls
`ClientConn.RoundTrip` once. A closed pooled connection can therefore produce a
502 for that request; it is removed, and later requests can establish new connections.
This deliberately favors the strict no-replay contract over seamless stale-socket
recovery. Idle connections are pruned on subsequent pool acquisitions after 60 seconds.
Headers have a 1 MiB limit; client headers have a ten-second deadline and upstream
response headers have a thirty-second deadline. Streaming bodies have no total
duration timeout; cancellation and shutdown terminate them.

TCP retry exists only in connection establishment. `connect_retries: 1` permits
two total TCP attempts, with the configured timeout per attempt. Between failed
connects, automatic mode re-resolves WSL using address/route evidence without
additional TCP probes or resetting the retry budget. WSL discovery time is
additional to the connect timeout. Request cancellation interrupts discovery,
connection establishment and retry delays. After a connection is established,
write errors, response errors and HTTP statuses never cause a replay, including
POST requests with idempotency headers.

Counters reset on each start: Requests counts incoming HTTP requests; Retries
counts additional TCP attempts; Failures counts reverse-proxy and response-stream
errors. Upstream HTTP error statuses are forwarded and are not transport failures.
Diagnostic probes are separate from these counters. No request bodies, query
strings or credentials are logged.

## Tests and troubleshooting

Run the Linux tests with the race detector and `go vet` from Windows:

```powershell
.\build.cmd test
.\build.cmd
```

Tests cover strict config parsing, replacement and concurrent saves, validation,
WSL encodings and command arguments, route/interface selection, manual TCP probes,
connect retry budgets, cancellation, HTTP forwarding, keep-alive, concurrent
requests, no replay after upstream disconnect and port release. The race detector
runs with CGO enabled in the Linux Go image; the Windows build remains CGO-free.

Real Windows checks are still required: run the executable in Windows Terminal and
PowerShell, save/reload configuration, select the intended WSL distribution, start
and stop with real traffic, verify that POST is executed once during an upstream
disconnect, and test your NAT/mirrored/VPN network. To exercise Windows-specific
configuration replacement tests without installing Go on Windows, compile a test
executable from WSL and then run it in Windows:

```bash
docker run --rm --mount "type=bind,source=$PWD,target=/src" -w /src \
  -v wslproxy-go-mod:/go/pkg/mod \
  -e CGO_ENABLED=0 -e GOOS=windows -e GOARCH=amd64 \
  golang:1.27.3 go test -mod=readonly -c -o bin/config.test.exe ./internal/config
```

```powershell
.\bin\config.test.exe -test.v
```

Common failures:

- **Docker unavailable:** start Docker Desktop with Linux containers; enable WSL
  integration if the source is in WSL.
- **Module lookup disabled:** restore the dependency volume with the download
  command above. Image download alone does not populate the module cache.
- **No distribution:** install/select the intended WSL distribution, then refresh
  the list. Linux must have `iproute2` for automatic address discovery.
- **No reachable upstream:** ensure the HTTP service is running on the selected
  port and a WSL interface reachable from Windows. A service bound only to Linux
  loopback may be inaccessible through its NAT address. Review VPN and firewall
  rules manually; the application does not change them.
- **Multiple addresses:** use diagnostics and explicitly set a manual IP.
- **Listen failed:** choose an available local IPv4/port; check occupied or reserved
  Windows ports. Binding `0.0.0.0` exposes the listener on all IPv4 interfaces,
  subject to Windows Firewall. The proxy adds no authentication or TLS.
- **Old manual IP:** clear the manual field to re-enable detection after WSL restarts.

## Code organization

`cmd/proxy` selects TUI or `serve`; `internal/app` coordinates their shared
lifecycle. `internal/config` owns persistence and validation; `internal/wsl` owns
command execution and discovery; `internal/proxy` owns HTTP, connection reuse,
retry and counters. `internal/tui` owns Bubble Tea state, Bubbles lists/inputs,
Lip Gloss rendering and a bounded in-memory event log. No application HTTP or
retry implementation is embedded in a view.

Pinned libraries: `charm.land/bubbletea/v2 v2.1.0`,
`charm.land/bubbles/v2 v2.2.1`, `charm.land/lipgloss/v2 v2.0.6`.
These are the current v2 module paths for the Charmbracelet projects; `go.mod`
and `go.sum` pin the dependency graph.
