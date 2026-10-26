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

// Client provides a client for the chain sync protocol only
type Client struct {
	logger  Logger
	options Options
	pool    *connPool // nil unless WithConnectionPool is set
}

// New returns a new Client
func New(opts ...Option) *Client {
	options := buildOptions(opts...)
	logger := options.logger.With(KV("service", "ogmios"))

	client := &Client{
		logger:  logger,
		options: options,
	}
	if options.poolSize > 0 {
		client.pool = newConnPool(options.poolSize, options.poolIdleTimeout)
	}
	return client
}

// CloseIdleConnections closes the connections a pooled client is holding
// open between calls. The client stays usable: later calls dial and pool
// connections again. It does nothing for a client without a pool.
func (c *Client) CloseIdleConnections() {
	if c.pool != nil {
		c.pool.closeIdle()
	}
}
