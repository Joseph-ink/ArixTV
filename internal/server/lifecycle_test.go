package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/config"
)

type notificationRecorder struct {
	ready     chan struct{}
	heartbeat chan struct{}
	mu        sync.Mutex
	states    []string
	interval  time.Duration
	fail      bool
}

func (n *notificationRecorder) WatchdogInterval() time.Duration { return n.interval }
func (n *notificationRecorder) Notify(state string) error {
	if n.fail {
		return errors.New("PRIVATE_PLACEHOLDER notification error")
	}
	n.mu.Lock()
	n.states = append(n.states, state)
	n.mu.Unlock()
	if state == "READY=1" {
		close(n.ready)
	}
	if state == "WATCHDOG=1" {
		select {
		case n.heartbeat <- struct{}{}:
		default:
		}
	}
	return nil
}

func TestServeGracefulShutdownAndNotifications(t *testing.T) {
	f := setup(t, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifier := &notificationRecorder{ready: make(chan struct{}), heartbeat: make(chan struct{}, 1), interval: time.Millisecond}
	result := make(chan error, 1)
	go func() { result <- f.app.Serve(ctx, []Binding{{Loopback, ln}}, notifier) }()
	await(t, notifier.ready)
	await(t, notifier.heartbeat)
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	started, cancelled := make(chan struct{}), make(chan struct{})
	f.runtime.resolve = func(ctx context.Context, _ channel.Channel) (channel.Stream, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return channel.Stream{}, ctx.Err()
	}
	requestDone := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + ln.Addr().String() + f.public("play/mock/direct"))
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != 503 {
				err = errors.New("unexpected shutdown response")
			}
		}
		requestDone <- err
	}()
	await(t, started)
	cancel()
	await(t, cancelled)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not terminate")
	}
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request remained active")
	}
	notifier.mu.Lock()
	states := append([]string(nil), notifier.states...)
	notifier.mu.Unlock()
	if states[0] != "READY=1" || states[len(states)-1] != "STOPPING=1" {
		t.Fatal("service lifecycle notifications")
	}
	if len(f.app.connections) != 0 || len(f.app.resolvers) != 0 {
		t.Fatal("resource permit leaked on shutdown")
	}
	if _, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second); err == nil {
		t.Fatal("listener still accepting")
	}
}

func TestStartupBindRollback(t *testing.T) {
	f := setup(t, nil)
	var opened net.Listener
	calls := 0
	f.app.listen = func(string, string) (net.Listener, error) {
		calls++
		if calls == 1 {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			opened = ln
			return ln, err
		}
		return nil, errors.New("PRIVATE_PLACEHOLDER address error")
	}
	err := f.app.Run(context.Background())
	if err == nil || strings.Contains(err.Error(), "PRIVATE_PLACEHOLDER") {
		t.Fatal("unsafe bind error", err)
	}
	if calls != 2 {
		t.Fatal("unexpected startup sequence")
	}
	if _, err := opened.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatal("partial startup listener remained open")
	}
}

func TestReadinessFailureClosesListeners(t *testing.T) {
	f := setup(t, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	err = f.app.Serve(context.Background(), []Binding{{Loopback, ln}}, &notificationRecorder{fail: true})
	if err == nil || strings.Contains(err.Error(), "PRIVATE_PLACEHOLDER") {
		t.Fatal("unsafe readiness failure")
	}
	if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatal("listener leaked after readiness failure")
	}
}

func TestManagedSnapshotReload(t *testing.T) {
	f := setup(t, nil)
	ticks := make(chan time.Time)
	f.app.reloadTicks = ticks
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifier := &notificationRecorder{ready: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- f.app.Serve(ctx, []Binding{{Loopback, ln}}, notifier) }()
	await(t, notifier.ready)
	next := f.candidate
	next.AttemptedAt = next.CompletedAt.Add(time.Minute)
	next.CompletedAt = next.AttemptedAt.Add(time.Second)
	if _, err := f.snapshots.Commit(next); err != nil {
		t.Fatal(err)
	}
	// A second unbuffered tick is accepted only after the first reload finishes.
	for range 2 {
		select {
		case ticks <- testTime:
		case <-time.After(3 * time.Second):
			t.Fatal("reload event loop stalled")
		}
	}
	if view := f.app.catalog.view.Load(); view.providers[0].Generation != 2 {
		t.Fatal("managed reload did not publish completed generation")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reload loop did not stop")
	}
}

// Fake listener/connection sequence proves capacity is released and rejected
// sockets get closed before dispatch; no timing/polling assumptions are needed.
type sequenceListener struct {
	conns []net.Conn
	index int
}

func (l *sequenceListener) Accept() (net.Conn, error) {
	if l.index == len(l.conns) {
		return nil, net.ErrClosed
	}
	c := l.conns[l.index]
	l.index++
	return c, nil
}
func (*sequenceListener) Close() error   { return nil }
func (*sequenceListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8787} }
func TestConnectionBound(t *testing.T) {
	first, peer1 := net.Pipe()
	second, peer2 := net.Pipe()
	defer peer1.Close()
	defer peer2.Close()
	inner := &sequenceListener{conns: []net.Conn{first, second}}
	l := &cappedListener{Listener: inner, permits: make(chan struct{}, 1), closed: make(chan struct{})}
	accepted, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatal("excess connection dispatched")
	}
	if len(l.permits) != 1 {
		t.Fatal("permit count")
	}
	accepted.Close()
	accepted.Close()
	if len(l.permits) != 0 {
		t.Fatal("permit not released exactly once")
	}
	l.Close()
}

func TestServeRejectsWildcardAndDisabledLAN(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.LAN.Enabled = false })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.app.Serve(context.Background(), []Binding{{LAN, ln}}, &notificationRecorder{}); err == nil {
		t.Fatal("disabled LAN binding accepted")
	}
	g := setup(t, nil)
	fake := &addressListener{address: &net.TCPAddr{IP: net.IPv4zero, Port: 8787}}
	if err := g.app.Serve(context.Background(), []Binding{{Loopback, fake}}, &notificationRecorder{}); err == nil {
		t.Fatal("wildcard loopback binding accepted")
	}
}

type addressListener struct{ address net.Addr }

func (l *addressListener) Addr() net.Addr          { return l.address }
func (*addressListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (*addressListener) Close() error              { return nil }
