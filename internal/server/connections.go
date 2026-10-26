package server

import (
	"net"
	"sync"
)

// One permit per accepted TCP connection, shared across all listeners. Excess
// connections are closed before net/http creates a handler goroutine. Idle
// listening sockets do not reserve permits or starve the other listeners.
type cappedListener struct {
	net.Listener
	permits chan struct{}
	closed  chan struct{}
	once    sync.Once
}
type cappedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (l *cappedListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case <-l.closed:
			c.Close()
			return nil, net.ErrClosed
		default:
		}
		select {
		case l.permits <- struct{}{}:
			return &cappedConn{Conn: c, release: func() { <-l.permits }}, nil
		default:
			c.Close()
		}
	}
}
func (l *cappedListener) Close() error {
	var err error
	l.once.Do(func() { close(l.closed); err = l.Listener.Close() })
	return err
}
func (c *cappedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
