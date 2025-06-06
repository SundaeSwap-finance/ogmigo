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
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync"
	"github.com/tj/assert"
)

func TestClient_ChainSync(t *testing.T) {
	endpoint := os.Getenv("OGMIOS")
	if endpoint == "" {
		t.SkipNow()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var (
		p       = message.NewPrinter(language.English)
		counter int64
		read    int64
	)

	client := New(WithEndpoint(endpoint))
	var callback ChainSyncFunc = func(ctx context.Context, data []byte) error {
		var response chainsync.ResponsePraos
		decoder := json.NewDecoder(
			bytes.NewReader(data),
		) // use decoder to check for unknown fields
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&response)
		if err != nil {
			fmt.Println(string(data))
			t.Fatalf("got %v; want nil", err)
		}

		read += int64(len(data))
		if v := atomic.AddInt64(&counter, 1); v%1e3 == 0 {
			var blockNo uint64
			nbr := response.MustNextBlockResult()
			if nbr.Direction == chainsync.RollForwardString {
				blockNo = nbr.Block.Height
			}
			log.Printf(
				"read: block=%v, n=%v, read=%v",
				blockNo,
				p.Sprintf("%d", v),
				p.Sprintf("%d", read),
			)
		}

		return nil
	}

	closer, err := client.ChainSync(ctx, callback, WithStore(echoStore{}))
	if err != nil {
		t.Fatalf("got %v; want nil", err)
	}

	<-time.After(5 * time.Second)
	if err := closer.Close(); err != nil {
		t.Fatalf("got %v; want nil", err)
	}
}

type echoStore struct {
}

func (e echoStore) Save(_ context.Context, p chainsync.Point) error {
	fmt.Print("Save => ")
	return json.NewEncoder(os.Stdout).Encode(p)
}

func (e echoStore) Load(context.Context) (chainsync.Points, error) {
	return nil, nil
}

type mockStore struct {
	pp chainsync.Points
}

func (m mockStore) Save(_ context.Context, p chainsync.Point) error {
	return nil
}

func (m mockStore) Load(context.Context) (chainsync.Points, error) {
	return m.pp, nil
}

func Test_getInit(t *testing.T) {
	ctx := context.Background()
	p1 := chainsync.PointStruct{
		Height: nil,
		ID:     "hash",
		Slot:   456,
	}
	p2 := chainsync.PointStruct{
		Height: nil,
		ID:     "hash",
		Slot:   654,
	}

	t.Run("from store", func(t *testing.T) {
		store := mockStore{
			pp: chainsync.Points{p1.Point()},
		}
		points, err := getInit(ctx, store, p2.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		want := `{"id":{"step":"INIT"},"jsonrpc":"2.0","method":"findIntersection","params":{"points":[{"id":"hash","slot":456}]}}`
		assert.EqualValues(t, string(points), want)
	})

	t.Run("from points", func(t *testing.T) {
		store := mockStore{}
		points, err := getInit(ctx, store, p1.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		want := `{"id":{"step":"INIT"},"jsonrpc":"2.0","method":"findIntersection","params":{"points":[{"id":"hash","slot":456}]}}`
		assert.EqualValues(t, string(points), want)
	})
}
