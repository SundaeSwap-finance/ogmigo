// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ogmigo

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// connPool holds idle WebSocket connections for reuse by request/response
// calls. A connection is either idle in the pool or checked out by exactly
// one call, so no two calls ever share a connection's reads or writes.
type connPool struct {
	maxIdle     int
	idleTimeout time.Duration

	mu sync.Mutex
	// idle is ordered oldest first; connections are reused newest first so
	// the ones that go unused age out.
	idle []idleConn
	// timer closes expired connections when no call comes along to do it.
	// It is pending whenever idle is not empty.
	timer *time.Timer
}

type idleConn struct {
	conn  *websocket.Conn
	since time.Time
}

func newConnPool(maxIdle int, idleTimeout time.Duration) *connPool {
	return &connPool{
		maxIdle:     maxIdle,
		idleTimeout: idleTimeout,
	}
}

// get checks out the most recently used idle connection, or returns nil when
// none is left that has not expired.
func (p *connPool) get() *websocket.Conn {
	p.mu.Lock()
	expired := p.pruneLocked(time.Now())
	var conn *websocket.Conn
	if n := len(p.idle); n > 0 {
		conn = p.idle[n-1].conn
		p.idle[n-1] = idleConn{}
		p.idle = p.idle[:n-1]
	}
	p.mu.Unlock()
	closeAll(expired)
	return conn
}

// put returns a connection that is in a clean state - no request
// outstanding, no reply unread - to the pool, or closes it if the pool is
// full.
func (p *connPool) put(conn *websocket.Conn) {
	p.mu.Lock()
	// Taking the time under the lock keeps idle ordered oldest first.
	now := time.Now()
	expired := p.pruneLocked(now)
	if len(p.idle) < p.maxIdle {
		p.idle = append(p.idle, idleConn{conn: conn, since: now})
		conn = nil
		if len(p.idle) == 1 {
			p.scheduleLocked(p.idleTimeout)
		}
	}
	p.mu.Unlock()
	closeAll(expired)
	if conn != nil {
		_ = conn.Close()
	}
}

// expire closes the connections that have expired, and runs again when the
// oldest remaining one is due to.
func (p *connPool) expire() {
	p.mu.Lock()
	now := time.Now()
	expired := p.pruneLocked(now)
	if len(p.idle) > 0 {
		p.scheduleLocked(p.idle[0].since.Add(p.idleTimeout).Sub(now))
	}
	p.mu.Unlock()
	closeAll(expired)
}

// scheduleLocked arranges for expire to run after d. A run left over from
// an earlier schedule is harmless: it finds nothing expired and reschedules.
func (p *connPool) scheduleLocked(d time.Duration) {
	if p.timer == nil {
		p.timer = time.AfterFunc(d, p.expire)
		return
	}
	p.timer.Reset(d)
}

// closeIdle closes every idle connection. Connections checked out at the
// time are pooled again when their calls finish.
func (p *connPool) closeIdle() {
	p.mu.Lock()
	idle := p.idle
	p.idle = nil
	p.mu.Unlock()
	for _, ic := range idle {
		_ = ic.conn.Close()
	}
}

func (p *connPool) idleCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idle)
}

// pruneLocked removes the connections that have been idle for idleTimeout or
// longer and returns them for the caller to close outside the lock.
func (p *connPool) pruneLocked(now time.Time) []*websocket.Conn {
	n := 0
	for n < len(p.idle) && now.Sub(p.idle[n].since) >= p.idleTimeout {
		n++
	}
	if n == 0 {
		return nil
	}
	expired := make([]*websocket.Conn, n)
	for i := range n {
		expired[i] = p.idle[i].conn
	}
	p.idle = append(p.idle[:0], p.idle[n:]...)
	return expired
}

func closeAll(conns []*websocket.Conn) {
	for _, conn := range conns {
		_ = conn.Close()
	}
}
