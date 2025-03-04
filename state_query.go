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
	"context"
	"encoding/json"
	"fmt"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync"
	"github.com/SundaeSwap-finance/ogmigo/ouroboros/shared"
	"github.com/SundaeSwap-finance/ogmigo/ouroboros/statequery"
)

func (c *Client) ChainTip(ctx context.Context) (chainsync.Point, error) {
	var (
		payload = makePayload("queryLedgerState/tip", Map{}, nil)
		content struct{ Result chainsync.Point }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return chainsync.Point{}, err
	}

	return content.Result, nil
}

func (c *Client) CurrentEpoch(ctx context.Context) (uint64, error) {
	var (
		payload = makePayload("queryLedgerState/epoch", Map{}, nil)
		content struct{ Result uint64 }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return 0, err
	}

	return content.Result, nil
}

func (c *Client) CurrentProtocolParameters(ctx context.Context) (json.RawMessage, error) {
	var (
		payload = makePayload("queryLedgerState/protocolParameters", Map{}, nil)
		content struct{ Result json.RawMessage }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return nil, err
	}

	return content.Result, nil
}

func (c *Client) GenesisConfig(ctx context.Context, era string) (json.RawMessage, error) {
	var (
		payload = makePayload("queryNetwork/genesisConfiguration", Map{"era": era}, nil)
		content struct{ Result json.RawMessage }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return nil, err
	}

	return content.Result, nil
}

func (c *Client) StartTime(ctx context.Context) (string, error) {
	var (
		payload = makePayload("queryNetwork/startTime", nil, nil)
		content struct{ Result string }
	)
	if err := c.query(ctx, payload, &content); err != nil {
		return "", err
	}
	return content.Result, nil
}

type EraHistory struct {
	Summaries []EraSummary
}

type EraSummary struct {
	Start      EraBound      `json:"start"`
	End        EraBound      `json:"end"`
	Parameters EraParameters `json:"parameters"`
}

type EraBound struct {
	Time  statequery.EraSeconds `json:"time"`
	Slot  uint64                `json:"slot"`
	Epoch uint64                `json:"epoch"`
}

type EraParameters struct {
	EpochLength uint64                     `json:"epochLength"`
	SlotLength  statequery.EraMilliseconds `json:"slotLength"`
	SafeZone    uint64                     `json:"safeZone"`
}

func (c *Client) EraSummaries(ctx context.Context) (*EraHistory, error) {
	var (
		payload = makePayload("queryLedgerState/eraSummaries", Map{}, nil)
		content struct{ Result json.RawMessage }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return nil, err
	}

	var summaries []EraSummary
	if err := json.Unmarshal(content.Result, &summaries); err != nil {
		return nil, err
	}

	return &EraHistory{
		Summaries: summaries,
	}, nil
}

func SlotToElapsedMilliseconds(history *EraHistory, slot uint64) uint64 {
	totalMsElapsed := uint64(0)
	for _, summary := range history.Summaries {
		intervalEnd := uint64(0)
		if summary.End.Slot < slot {
			// The era has passed
			intervalEnd = summary.End.Slot
		} else if summary.Start.Slot < slot {
			// The era is in progress
			intervalEnd = slot
		} else {
			// The era is in the future
			continue
		}
		// Compute the number of elapsed milliseconds for this era
		slotsElapsedThisEra := intervalEnd - summary.Start.Slot
		slotLengthMs := summary.Parameters.SlotLength.Milliseconds.Uint64()
		msElapsedThisEra := slotsElapsedThisEra * slotLengthMs
		totalMsElapsed += msElapsedThisEra
	}
	return totalMsElapsed
}

func (c *Client) EraStart(ctx context.Context) (statequery.EraStart, error) {
	var (
		payload = makePayload("queryLedgerState/eraStart", Map{}, nil)
		content struct{ Result statequery.EraStart }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return statequery.EraStart{}, err
	}

	return content.Result, nil
}

func (c *Client) UtxosByAddress(ctx context.Context, addresses ...string) ([]shared.Utxo, error) {
	var (
		payload = makePayload("queryLedgerState/utxo", Map{"addresses": addresses}, nil)
		content struct{ Result []shared.Utxo }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return nil, fmt.Errorf("failed to query utxos by address: %w", err)
	}

	return content.Result, nil
}

func (c *Client) UtxosByTxIn(ctx context.Context, txIns ...chainsync.TxInQuery) ([]shared.Utxo, error) {
	var (
		payload = makePayload("queryLedgerState/utxo", Map{"outputReferences": txIns}, nil)
		content struct{ Result []shared.Utxo }
	)

	if err := c.query(ctx, payload, &content); err != nil {
		return nil, fmt.Errorf("failed to query utxos by address: %w", err)
	}

	return content.Result, nil
}
