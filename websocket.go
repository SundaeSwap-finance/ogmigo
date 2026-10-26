// Copyright 2021 Matt Ho
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"syscall"

	"github.com/gorilla/websocket"
)

var fault = []byte(`jsonwsp/fault`)

// query sends one request and decodes its reply into v. With a connection
// pool the call reuses an idle connection when one is available; otherwise
// it dials a connection for this call alone.
func (c *Client) query(
	ctx context.Context,
	payload any,
	v any,
) error {
	conn, reused, err := c.acquireConn(ctx)
	if err != nil {
		return err
	}
	raw, delivered, keep, err := roundTrip(ctx, conn, payload)
	if err != nil && reused && ctx.Err() == nil &&
		(!delivered || isConnClosed(err)) {
		// The pooled connection was closed while it sat idle, typically by
		// the server or a proxy reaping it. Retry once on a new connection
		// rather than surface a failure the caller did not cause.
		conn, err = c.dial(ctx)
		if err != nil {
			return err
		}
		raw, _, keep, err = roundTrip(ctx, conn, payload)
	}
	if err != nil {
		return err
	}
	if keep {
		c.releaseConn(conn)
	}

	if bytes.Contains(raw, fault) {
		var e Error
		if err := json.Unmarshal(raw, &e); err != nil {
			return fmt.Errorf("failed to decode error: %w", err)
		}
		return e
	}

	if v != nil {
		if err := json.Unmarshal(raw, v); err != nil {
			return fmt.Errorf("failed to unmarshal contents: %w", err)
		}
	}

	return nil
}

// acquireConn returns a connection for one call and whether it came from
// the pool.
func (c *Client) acquireConn(
	ctx context.Context,
) (conn *websocket.Conn, reused bool, err error) {
	if c.pool != nil {
		if conn := c.pool.get(); conn != nil {
			return conn, true, nil
		}
	}
	conn, err = c.dial(ctx)
	return conn, false, err
}

// releaseConn hands back a connection whose call completed cleanly.
func (c *Client) releaseConn(conn *websocket.Conn) {
	if c.pool != nil {
		c.pool.put(conn)
		return
	}
	_ = conn.Close()
}

func (c *Client) dial(ctx context.Context) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(
		ctx,
		c.options.endpoint,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to connect to ogmios, %v: %w",
			c.options.endpoint,
			err,
		)
	}
	return conn, nil
}

// roundTrip writes one request to conn and reads its reply. delivered
// reports whether the request was written. keep reports whether conn is
// still open with nothing left to read, and so may be reused; when it is
// false, roundTrip has closed conn. Cancelling ctx closes conn to unblock
// the call, which then returns ctx.Err().
func roundTrip(
	ctx context.Context,
	conn *websocket.Conn,
	payload any,
) (raw json.RawMessage, delivered, keep bool, err error) {
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	fail := func(err error) (json.RawMessage, bool, bool, error) {
		stop()
		_ = conn.Close()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, delivered, false, ctxErr
		}
		return nil, delivered, false, err
	}

	if err := conn.WriteJSON(payload); err != nil {
		return fail(fmt.Errorf("failed to submit request: %w", err))
	}
	delivered = true

	if err := conn.ReadJSON(&raw); err != nil {
		return fail(fmt.Errorf("failed to read json response: %w", err))
	}

	// The reply is complete. If ctx was cancelled just as it arrived, the
	// connection has been closed, but the reply is still good.
	return raw, true, stop(), nil
}

// isConnClosed reports whether err shows the peer closed or reset the
// connection.
func isConnClosed(err error) bool {
	var closeErr *websocket.CloseError
	return errors.As(err, &closeErr) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}
