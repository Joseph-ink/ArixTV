package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/servicenotify"
)

type Binding struct {
	Kind     ListenerKind
	Listener net.Listener
}

// Run opens every configured listener before starting any HTTP service. A failed
// LAN bind closes already opened loopback sockets and returns a safe error.
func (a *App) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	var bindings []Binding
	defer func() {
		for _, b := range bindings {
			b.Listener.Close()
		}
	}()
	open := func(kind ListenerKind, ip string) error {
		ln, err := a.listen("tcp", net.JoinHostPort(ip, strconv.Itoa(a.cfg.Server.Port)))
		if err != nil {
			return errors.New("cannot bind configured " + string(kind) + " HTTP listener")
		}
		bindings = append(bindings, Binding{Kind: kind, Listener: ln})
		return nil
	}
	if err := open(Loopback, a.cfg.Server.LoopbackAddress); err != nil {
		return err
	}
	if a.cfg.LAN.Enabled {
		for _, ip := range a.cfg.LAN.ListenAddresses {
			if err := open(LAN, ip); err != nil {
				return err
			}
		}
	}
	notifier, err := servicenotify.FromEnv()
	if err != nil {
		return err
	}
	return a.Serve(ctx, bindings, notifier)
}

type safeHTTPLog struct{ app *App }

func (s safeHTTPLog) Write(p []byte) (int, error) {
	s.app.logger.Warn("HTTP transport error", "code", "http_transport_error")
	return len(p), nil
}

func (a *App) httpServer(ctx context.Context, kind ListenerKind) *http.Server {
	return &http.Server{Handler: a.Handler(kind), ReadHeaderTimeout: duration(a.cfg.HTTP.ReadHeaderTimeout), ReadTimeout: duration(a.cfg.HTTP.ReadTimeout), WriteTimeout: duration(a.cfg.HTTP.WriteTimeout), IdleTimeout: duration(a.cfg.HTTP.IdleTimeout), MaxHeaderBytes: a.cfg.HTTP.MaxHeaderBytes, BaseContext: func(net.Listener) context.Context { return ctx }, ErrorLog: log.New(safeHTTPLog{a}, "", 0)}
}

// Serve accepts pre-bound listeners for embedding and deterministic local tests.
// Loopback bindings are checked even here, not only in the CLI binding path.
// The App is single-use; all listeners/goroutines are owned until return.
func (a *App) Serve(ctx context.Context, bindings []Binding, notifier servicenotify.Notifier) error {
	if !a.serving.CompareAndSwap(false, true) {
		return errors.New("HTTP application already served")
	}
	if len(bindings) == 0 {
		return errors.New("no HTTP listeners configured")
	}
	defer func() {
		for _, b := range bindings {
			if b.Listener != nil {
				b.Listener.Close()
			}
		}
	}()
	for _, b := range bindings {
		if b.Listener == nil {
			return errors.New("invalid HTTP listener")
		}
		addr, err := netip.ParseAddrPort(b.Listener.Addr().String())
		if err != nil || (b.Kind != Loopback && b.Kind != LAN) || b.Kind == Loopback && !addr.Addr().Unmap().IsLoopback() {
			return errors.New("listener does not satisfy network policy")
		}
		if b.Kind == LAN && !a.cfg.LAN.Enabled {
			return errors.New("LAN listener is disabled")
		}
		if b.Kind == LAN {
			matched := false
			for _, value := range a.cfg.LAN.ListenAddresses {
				ip, _ := netip.ParseAddr(value)
				if addr.Addr().Unmap() == ip.Unmap() {
					matched = true
				}
			}
			if !matched || addr.Addr().IsUnspecified() {
				return errors.New("LAN listener address is not configured")
			}
		}
	}
	if notifier == nil {
		notifier, _ = servicenotify.FromEnv()
	}
	if notifier == nil {
		return errors.New("invalid service notifier")
	}
	if notifier.WatchdogInterval() < 0 {
		return errors.New("invalid watchdog interval")
	}
	base, cancel := context.WithCancel(ctx)
	defer cancel()
	servers := make([]*http.Server, 0, len(bindings))
	listeners := make([]net.Listener, 0, len(bindings))
	failed := make(chan error, len(bindings))
	var workers sync.WaitGroup
	for _, b := range bindings {
		s := a.httpServer(base, b.Kind)
		ln := &cappedListener{Listener: b.Listener, permits: a.connections, closed: make(chan struct{})}
		servers = append(servers, s)
		listeners = append(listeners, ln)
		workers.Add(1)
		go func() { defer workers.Done(); failed <- s.Serve(ln) }()
	}
	var result error
	if err := notifier.Notify("READY=1"); err != nil {
		result = errors.New("service readiness notification failed")
	} else {
		a.logger.Info("HTTP service ready", "code", "ready", "listener_count", len(bindings))
		reloadC := a.reloadTicks
		if reloadC == nil {
			reload := time.NewTicker(duration(a.cfg.HTTP.ReloadInterval))
			defer reload.Stop()
			reloadC = reload.C
		}
		var heartbeat <-chan time.Time
		if interval := notifier.WatchdogInterval(); interval > 0 {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			heartbeat = ticker.C
		}
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case <-failed:
				result = errors.New("HTTP listener stopped unexpectedly")
				break loop
			case <-reloadC:
				if err := a.Reload(); err != nil {
					a.logger.Warn("catalog reload incomplete", "code", "catalog_degraded")
				}
			case <-heartbeat:
				if err := notifier.Notify("WATCHDOG=1"); err != nil {
					result = errors.New("service watchdog notification failed")
					break loop
				}
			}
		}
	}
	a.stopping.Store(true)
	cancel()
	for _, ln := range listeners {
		ln.Close()
	}
	if err := notifier.Notify("STOPPING=1"); err != nil {
		a.logger.Warn("service stopping notification failed", "code", "notify_stopping_failed")
	}
	shutdown, stop := context.WithTimeout(context.Background(), duration(a.cfg.HTTP.ShutdownTimeout))
	defer stop()
	for _, s := range servers {
		if err := s.Shutdown(shutdown); err != nil {
			s.Close()
			result = errors.New("HTTP graceful shutdown deadline exceeded")
		}
	}
	workers.Wait()
	return result
}
