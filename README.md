ogmigo
-------------------------

`ogmigo` is a go client for [ogmios](https://ogmios.dev).

This library is under heavy development, use at your own risk.

### Example

```go
package example

import (
	"context"
	"github.com/SundaeSwap-finance/ogmigo"
)

func example(ctx context.Context) error {
	var callback ogmigo.ChainSyncFunc = func(ctx context.Context, data []byte) error {
		// do work
		return nil
	}

	client := ogmigo.New(ogmigo.WithEndpoint("ws://example.com:1337"))
	closer, err := client.ChainSync(ctx, callback)
	if err != nil {
		return err
	}
	if err := closer.Close(); err != nil {
		return err
	}

	return nil
}
```

### Connection reuse

By default, each request/response call (state queries, `SubmitTx`,
`EvaluateTx`) dials a new WebSocket connection and closes it afterwards.
Clients that make frequent calls can keep connections open between calls
instead:

```go
client := ogmigo.New(
	ogmigo.WithEndpoint("ws://example.com:1337"),
	ogmigo.WithConnectionPool(4), // keep up to 4 idle connections
)
defer client.CloseIdleConnections()
```

Each pooled connection carries one call at a time. Idle connections are
closed after 30 seconds by default (`WithPoolIdleTimeout`). Chain sync and
mempool monitoring are unaffected; they hold their own connections.

State queries and transaction evaluation retry once on a fresh connection
if a reused connection is closed. Transaction submissions return the original
connection error unless `WithSubmissionRetries(true)` is set. Opting in can
return a duplicate rejection if the server accepted the first submission
before the connection failed.

### Submodules

`ogmigo` imports `ogmios` as a submodule for testing purposes. To fetch the submodules,

```bash
git submodule update --init --recursive
```

