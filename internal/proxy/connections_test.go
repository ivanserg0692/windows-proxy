package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func waitForConnections(t *testing.T, server *Server, want int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		got := server.Snapshot().ActiveConnections
		if got == want {
			return
		}
		if got < 0 || time.Now().After(deadline) {
			t.Fatalf("active connections = %d, want %d", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func dialTestClient(t *testing.T, server *Server) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp4", server.Address(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestConcurrentIdleConnectionsAndClose(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()
	server := startTestProxy(t, testConfig(t, upstream.Listener.Addr().String()))
	waitForConnections(t, server, 0)
	const count = 32
	connections := make([]net.Conn, count)
	var wg sync.WaitGroup
	for i := range connections {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp4", server.Address(), time.Second)
			if err != nil {
				t.Error(err)
				return
			}
			connections[i] = conn
		}(i)
	}
	wg.Wait()
	t.Cleanup(func() {
		for _, conn := range connections {
			if conn != nil {
				conn.Close()
			}
		}
	})
	if t.Failed() {
		t.FailNow()
	}
	waitForConnections(t, server, count)
	if server.Snapshot().Requests != 0 {
		t.Fatal("idle TCP connections must not register HTTP requests")
	}
	for index, batch := range [][]net.Conn{connections[:count/2], connections[count/2:]} {
		for _, conn := range batch {
			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				conn.Close()
			}(conn)
		}
		wg.Wait()
		waitForConnections(t, server, int64(count-(index+1)*(count/2)))
	}
}

func TestKeepAliveConnectionCountAndGracefulStop(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer upstream.Close()
	server := startTestProxy(t, testConfig(t, upstream.Listener.Addr().String()))
	conn := dialTestClient(t, server)
	reader := bufio.NewReader(conn)
	for i := 0; i < 3; i++ {
		if _, err := fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n"); err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("response: status %d, error %v", response.StatusCode, err)
		}
		waitForConnections(t, server, 1)
	}
	if server.Snapshot().Requests != 3 {
		t.Fatalf("requests: %+v", server.Snapshot())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if got := server.Snapshot().ActiveConnections; got != 0 {
		t.Fatalf("active connections after Stop = %d", got)
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		t.Fatalf("client connection not closed: %v", err)
	}
}

func TestWaitingForResponseConnectionCountAndForcedStop(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer upstream.Close()
	defer close(release)
	server := startTestProxy(t, testConfig(t, upstream.Listener.Addr().String()))
	conn := dialTestClient(t, server)
	if _, err := fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not receive request")
	}
	waitForConnections(t, server, 1)
	// Include a connection still waiting for its first request in forced cleanup.
	idle := dialTestClient(t, server)
	waitForConnections(t, server, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Stop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("forced Stop error = %v", err)
	}
	if got := server.Snapshot().ActiveConnections; got != 0 {
		t.Fatalf("active connections after forced Stop = %d", got)
	}
	for _, client := range []net.Conn{conn, idle} {
		if _, err := client.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
			t.Fatalf("client connection not closed: %v", err)
		}
	}
}
