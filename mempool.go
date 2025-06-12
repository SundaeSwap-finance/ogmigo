package ogmigo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

type MonitorMempool struct {
	cancel context.CancelFunc
	errs   chan error
	done   chan struct{}
	err    error
	logger Logger
}

func (c *MonitorMempool) Done() <-chan struct{} {
	return c.done
}

func (c *MonitorMempool) Err() <-chan error {
	return c.errs
}

func (c *MonitorMempool) Close() error {
	c.cancel()
	select {
	case v := <-c.errs:
		c.err = v
	default:
		// err already set
	}
	return c.err
}

type MonitorMempoolFunc func(ctx context.Context, data []*chainsync.Tx, slot uint64) error

type MonitorMempoolOptions struct {
	reconnect bool // reconnect to ogmios if connection drops
}

func WithMempoolReconnect(enabled bool) MonitorMempoolOption {
	return func(opts *MonitorMempoolOptions) {
		opts.reconnect = enabled
	}
}

func buildMonitorMempoolOptions(
	opts ...MonitorMempoolOption,
) MonitorMempoolOptions {
	var options MonitorMempoolOptions
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

type MonitorMempoolOption func(opts *MonitorMempoolOptions)

func (c *Client) MonitorMempool(
	ctx context.Context,
	callback MonitorMempoolFunc,
	opts ...MonitorMempoolOption,
) (*MonitorMempool, error) {
	options := buildMonitorMempoolOptions(opts...)

	done := make(chan struct{})
	errs := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(done)

		var (
			timeout = 10 * time.Second
			err     error
		)
		for {
			err = c.doMonitorMempool(ctx, callback, options)
			if err != nil && isTemporaryError(err) {
				if options.reconnect {
					c.options.logger.Info(
						"websocket connection error: will retry",
						KV("delay", timeout.Round(time.Millisecond).String()),
						KV("err", err.Error()),
					)

					select {
					case <-ctx.Done():
						return
					case <-time.After(timeout):
						continue
					}
				}
			}

			break
		}
		errs <- err
	}()

	return &MonitorMempool{
		cancel: cancel,
		errs:   errs,
		done:   done,
		logger: c.logger,
	}, nil
}

type MonitorState int

const (
	AcquireMempool MonitorState = iota
	NextTransaction
)

type AcquireMempoolResponse struct {
	Method string
	Result struct {
		Acquired string
		Slot     uint64
	}
}

type NextTransactionResponse struct {
	Method string
	Result struct {
		Transaction *chainsync.Tx
	}
}

func (c *Client) doMonitorMempool(
	ctx context.Context,
	callback MonitorMempoolFunc,
	options MonitorMempoolOptions,
) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.options.endpoint, nil)
	if err != nil {
		return fmt.Errorf(
			"failed to connect to ogmios, %v: %w",
			c.options.endpoint,
			err,
		)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		c.options.logger.Info("ogmigo mempool monitoring started")
		defer c.options.logger.Info("ogmigo mempool monitoring stopped")
		<-ctx.Done()
		return nil
	})

	var connState int64 // 0 - open, 1 - closing, 2 - closed
	group.Go(func() error {
		<-ctx.Done()
		atomic.AddInt64(&connState, 1)
		if err := conn.Close(); err != nil {
			return err
		}
		atomic.AddInt64(&connState, 1)
		return nil
	})

	// prime the pump
	ch := make(chan MonitorState)

	group.Go(func() error {
		nextTransaction := []byte(
			`{"jsonrpc":"2.0","method":"nextTransaction","params":{"fields":"all"},"id":{}}`,
		)
		acquireMempool := []byte(
			`{"jsonrpc":"2.0","method":"acquireMempool","id":{"step":"MEMPOOLINIT"}}`,
		)
		var todo MonitorState
		for {
			select {
			case <-ctx.Done():
				return nil
			case todo = <-ch:
				switch todo {
				case AcquireMempool:
					if err := conn.WriteMessage(websocket.TextMessage, acquireMempool); err != nil {
						var oe *net.OpError
						if ok := errors.As(err, &oe); ok {
							if v := atomic.LoadInt64(&connState); v > 0 {
								return nil // connection closed
							}
						}
						return fmt.Errorf(
							"failed to write acquireMempool: %w",
							err,
						)
					}
				case NextTransaction:
					if err := conn.WriteMessage(websocket.TextMessage, nextTransaction); err != nil {
						return fmt.Errorf(
							"failed to write nextTransaction: %w",
							err,
						)
					}
				default:
					return errors.New("invalid channel state")
				}
			}
		}
	})

	group.Go(func() error {
		ch <- AcquireMempool
		var transactions []*chainsync.Tx
		var slot uint64
		for n := uint64(1); ; n++ {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				var oe *net.OpError
				if ok := errors.As(err, &oe); ok {
					if v := atomic.LoadInt64(&connState); v > 0 {
						return nil // connection closed
					}
				}
				return fmt.Errorf("failed to read message from ogmios: %w", err)
			}

			switch messageType {
			case websocket.BinaryMessage:
				c.options.logger.Info("skipping unexpected binary message")
				continue

			case websocket.CloseMessage:
				return nil

			case websocket.PingMessage:
				if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
					return fmt.Errorf(
						"failed to respond with pong to ogmios: %w",
						err,
					)
				}
				continue

			case websocket.PongMessage:
				continue

			case websocket.TextMessage:
				// ok
			}

			var acquireMempoolResponse AcquireMempoolResponse
			acquireMempoolErr := json.Unmarshal(data, &acquireMempoolResponse)

			var nextTransactionResponse NextTransactionResponse
			nextTransactionErr := json.Unmarshal(data, &nextTransactionResponse)

			if acquireMempoolErr != nil && nextTransactionErr != nil {
				return fmt.Errorf(
					"couldn't parse response from ogmios: %w",
					errors.Join(acquireMempoolErr, nextTransactionErr),
				)
			}

			if acquireMempoolResponse.Method == "acquireMempool" &&
				acquireMempoolErr == nil {
				slot = acquireMempoolResponse.Result.Slot
				ch <- NextTransaction
			} else if nextTransactionResponse.Method == "nextTransaction" && nextTransactionResponse.Result.Transaction == nil {
				err := callback(ctx, transactions, slot)
				transactions = nil
				if err != nil {
					return fmt.Errorf("mempool monitoring stopped: callback failed: %w", err)
				}
				ch <- AcquireMempool
			} else {
				transactions = append(transactions, nextTransactionResponse.Result.Transaction)
				ch <- NextTransaction
			}
		}
	})
	return group.Wait()
}
