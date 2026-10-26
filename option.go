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

import "time"

// Options available to ogmios client
type Options struct {
	endpoint        string
	logger          Logger
	pipeline        int
	saveInterval    uint64
	poolSize        int
	poolIdleTimeout time.Duration
}

// Option to cardano client
type Option func(*Options)

// WithEndpoint allows ogmios endpoint to set; defaults to ws://127.0.0.1:1337
func WithEndpoint(endpoint string) Option {
	return func(opts *Options) {
		opts.endpoint = endpoint
	}
}

// WithInterval specifies how frequently to save checkpoints when reading
func WithInterval(n int) Option {
	return func(options *Options) {
		options.saveInterval = uint64(n)
	}
}

// WithLogger allows custom logger to be specified
func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

// WithPipeline allows number of pipelined ogmios requests to be provided
func WithPipeline(n int) Option {
	return func(opts *Options) {
		opts.pipeline = n
	}
}

// WithConnectionPool makes request/response calls (state queries, transaction
// submission and evaluation) reuse WebSocket connections instead of dialing a
// new one per call. Up to n idle connections are kept open between calls;
// concurrent calls beyond that dial extra connections, which are closed once
// the pool is full again. Each connection carries one call at a time. n <= 0
// leaves pooling off, the default.
//
// A pooled client holds connections open after a call returns. Call
// Client.CloseIdleConnections when the client is no longer needed.
//
// A call that fails because a reused connection had been closed while idle is
// retried once on a new connection. Should the server have acted on the
// request before the connection dropped, a retried SubmitTx reports the
// node's rejection of the duplicate rather than the original hash.
func WithConnectionPool(n int) Option {
	return func(opts *Options) {
		opts.poolSize = n
	}
}

// WithPoolIdleTimeout sets how long a pooled connection may sit idle before
// it is closed rather than reused; defaults to 30 seconds. Keep it below any
// idle timeout of a proxy or load balancer in front of Ogmios. It has no
// effect unless WithConnectionPool is also set.
func WithPoolIdleTimeout(d time.Duration) Option {
	return func(opts *Options) {
		opts.poolIdleTimeout = d
	}
}

func buildOptions(opts ...Option) Options {
	var options Options
	for _, opt := range opts {
		opt(&options)
	}
	if options.endpoint == "" {
		options.endpoint = "ws://127.0.0.1:1337"
	}
	if options.logger == nil {
		options.logger = DefaultLogger
	}
	if options.pipeline <= 0 {
		options.pipeline = 50
	}
	if options.saveInterval <= 0 {
		options.saveInterval = 2160
	}
	if options.poolIdleTimeout <= 0 {
		options.poolIdleTimeout = 30 * time.Second
	}
	return options
}
