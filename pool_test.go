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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// echoServer is a WebSocket server that answers each request with the
// request itself. It counts the connections it accepts, and its hooks let a
// test delay a reply or drop a connection after answering.
type echoServer struct {
	*httptest.Server
	accepted atomic.Int64
	open     atomic.Int64

	// delay, when set, is how long to wait before answering a request.
	delay time.Duration
	// closeAfterReply, when set, drops each connection once it has answered
	// one request, as a server or proxy reaping the connection would.
	closeAfterReply bool
}

func newEchoServer(t *testing.T, configure func(*echoServer)) *echoServer {
	t.Helper()
	s := &echoServer{}
	if configure != nil {
		configure(s)
	}
	upgrader := websocket.Upgrader{}
	s.Server = httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			conn, err := upgrader.Upgrade(w, req, nil)
			if err != nil {
				return
			}
			s.accepted.Add(1)
			s.open.Add(1)
			defer s.open.Add(-1)
			//nolint:errcheck
			defer conn.Close()
			for {
				mt, message, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if s.delay > 0 {
					time.Sleep(s.delay)
				}
				if err := conn.WriteMessage(mt, message); err != nil {
					return
				}
				if s.closeAfterReply {
					return
				}
			}
		}),
	)
	t.Cleanup(s.Close)
	return s
}

func (s *echoServer) endpoint() string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

// echoQuery sends a request carrying tag and checks the reply is the echo of
// that same request, which proves no reply was crossed with another call's.
func echoQuery(ctx context.Context, c *Client, tag string) error {
	var got Map
	if err := c.query(ctx, Map{"tag": tag}, &got); err != nil {
		return err
	}
	if got["tag"] != tag {
		return errors.New("got reply for " + toString(got["tag"]))
	}
	return nil
}

func toString(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func TestQuery_withoutPoolDialsPerCall(t *testing.T) {
	server := newEchoServer(t, nil)
	client := New(WithEndpoint(server.endpoint()))

	for range 3 {
		if err := echoQuery(context.Background(), client, "a"); err != nil {
			t.Fatalf("query: %v", err)
		}
	}
	if got := server.accepted.Load(); got != 3 {
		t.Fatalf("accepted %d connections; want 3", got)
	}
}

func TestQuery_poolReusesConnection(t *testing.T) {
	server := newEchoServer(t, nil)
	client := New(WithEndpoint(server.endpoint()), WithConnectionPool(1))
	t.Cleanup(client.CloseIdleConnections)

	for i := range 5 {
		tag := string(rune('a' + i))
		if err := echoQuery(context.Background(), client, tag); err != nil {
			t.Fatalf("query %d: %v", i, err)
		}
	}
	if got := server.accepted.Load(); got != 1 {
		t.Fatalf("accepted %d connections; want 1", got)
	}
}

func TestQuery_poolConcurrentCallsGetTheirOwnReplies(t *testing.T) {
	server := newEchoServer(t, func(s *echoServer) {
		s.delay = 5 * time.Millisecond
	})
	const poolSize = 4
	client := New(
		WithEndpoint(server.endpoint()),
		WithConnectionPool(poolSize),
	)
	t.Cleanup(client.CloseIdleConnections)

	const workers, calls = 16, 10
	var wg sync.WaitGroup
	errs := make(chan error, workers*calls)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range calls {
				tag := string(rune('A'+w)) + string(rune('0'+i))
				if err := echoQuery(
					context.Background(), client, tag,
				); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("query: %v", err)
	}

	if got := client.pool.idleCount(); got > poolSize {
		t.Fatalf("pool retains %d connections; want at most %d", got, poolSize)
	}
	if got := server.accepted.Load(); got >= workers*calls {
		t.Fatalf("accepted %d connections; want reuse", got)
	}
}

func TestQuery_poolDiscardsConnectionOfCancelledCall(t *testing.T) {
	server := newEchoServer(t, func(s *echoServer) {
		s.delay = 200 * time.Millisecond
	})
	client := New(WithEndpoint(server.endpoint()), WithConnectionPool(1))
	t.Cleanup(client.CloseIdleConnections)

	ctx, cancel := context.WithTimeout(
		context.Background(), 20*time.Millisecond,
	)
	defer cancel()
	err := echoQuery(ctx, client, "abandoned")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v; want context.DeadlineExceeded", err)
	}
	if got := client.pool.idleCount(); got != 0 {
		t.Fatalf("pool retains %d connections; want 0", got)
	}

	// The late reply to the abandoned call must not be read as this one's.
	if err := echoQuery(context.Background(), client, "fresh"); err != nil {
		t.Fatalf("query after cancel: %v", err)
	}
	if got := server.accepted.Load(); got != 2 {
		t.Fatalf("accepted %d connections; want 2", got)
	}
}

func TestQuery_poolRetriesConnectionClosedWhileIdle(t *testing.T) {
	server := newEchoServer(t, func(s *echoServer) {
		s.closeAfterReply = true
	})
	client := New(WithEndpoint(server.endpoint()), WithConnectionPool(1))
	t.Cleanup(client.CloseIdleConnections)

	if err := echoQuery(context.Background(), client, "a"); err != nil {
		t.Fatalf("first query: %v", err)
	}
	waitFor(t, func() bool { return server.open.Load() == 0 })

	// The pooled connection is dead; the call must succeed on a new one.
	if err := echoQuery(context.Background(), client, "b"); err != nil {
		t.Fatalf("query on reaped connection: %v", err)
	}
	if got := server.accepted.Load(); got != 2 {
		t.Fatalf("accepted %d connections; want 2", got)
	}
}

func TestQuery_poolDoesNotRetryFreshConnection(t *testing.T) {
	var attempts atomic.Int64
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, req, nil)
			if err != nil {
				return
			}
			attempts.Add(1)
			_, _, _ = conn.ReadMessage()
			_ = conn.Close()
		}),
	)
	t.Cleanup(server.Close)
	client := New(
		WithEndpoint("ws"+strings.TrimPrefix(server.URL, "http")),
		WithConnectionPool(1),
	)
	t.Cleanup(client.CloseIdleConnections)

	if err := echoQuery(context.Background(), client, "a"); err == nil {
		t.Fatal("got nil error; want read failure")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("made %d attempts; want 1", got)
	}
}

func TestQuery_poolDropsExpiredIdleConnection(t *testing.T) {
	server := newEchoServer(t, nil)
	client := New(
		WithEndpoint(server.endpoint()),
		WithConnectionPool(1),
		WithPoolIdleTimeout(10*time.Millisecond),
	)
	t.Cleanup(client.CloseIdleConnections)

	if err := echoQuery(context.Background(), client, "a"); err != nil {
		t.Fatalf("first query: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := echoQuery(context.Background(), client, "b"); err != nil {
		t.Fatalf("second query: %v", err)
	}
	if got := server.accepted.Load(); got != 2 {
		t.Fatalf("accepted %d connections; want 2", got)
	}
	// The expired connection is closed, not leaked.
	waitFor(t, func() bool { return server.open.Load() == 1 })
}

func TestQuery_poolKeepsConnectionAfterFault(t *testing.T) {
	server := newEchoServer(t, nil)
	client := New(WithEndpoint(server.endpoint()), WithConnectionPool(1))
	t.Cleanup(client.CloseIdleConnections)

	// The echo of this request is a v5 fault reply: a complete message, so
	// the connection is still in a clean state.
	fault := Map{"type": "jsonwsp/fault", "fault": Map{"string": "boom"}}
	var e Error
	if err := client.query(context.Background(), fault, nil); !errors.As(
		err, &e,
	) {
		t.Fatalf("got %v; want Error", err)
	}
	if err := echoQuery(context.Background(), client, "a"); err != nil {
		t.Fatalf("query after fault: %v", err)
	}
	if got := server.accepted.Load(); got != 1 {
		t.Fatalf("accepted %d connections; want 1", got)
	}
}

func TestCloseIdleConnections(t *testing.T) {
	server := newEchoServer(t, nil)
	client := New(WithEndpoint(server.endpoint()), WithConnectionPool(2))

	if err := echoQuery(context.Background(), client, "a"); err != nil {
		t.Fatalf("query: %v", err)
	}
	client.CloseIdleConnections()
	waitFor(t, func() bool { return server.open.Load() == 0 })

	// The pool stays usable after its idle connections are closed.
	if err := echoQuery(context.Background(), client, "b"); err != nil {
		t.Fatalf("query after close: %v", err)
	}
	if got := server.accepted.Load(); got != 2 {
		t.Fatalf("accepted %d connections; want 2", got)
	}
	client.CloseIdleConnections()
}

func TestCloseIdleConnections_withoutPool(t *testing.T) {
	New().CloseIdleConnections()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met before deadline")
		}
		time.Sleep(time.Millisecond)
	}
}
