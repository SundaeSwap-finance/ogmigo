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
	"sort"
	"strings"

	"github.com/buger/jsonparser"
)

type Response struct {
	Transaction ResponseTx `json:"transaction,omitempty" dynamodbav:"transaction,omitempty"`
}

type ResponseTx struct {
	ID string `json:"id,omitempty" dynamodbav:"id,omitempty"`
}

type SubmitTx struct {
	Cbor string `json:"cbor"`
}

// type Response struct {
// 	Type        string
// 	Version     string
// 	ServiceName string `json:"servicename"`
// 	MethodName  string `json:"methodname"`
// 	Reflection  interface{}
// 	Result      json.RawMessage
// }

// SubmitTx submits the transaction via ogmios
// https://ogmios.dev/mini-protocols/local-tx-submission/
func (c *Client) SubmitTx(
	ctx context.Context,
	data string,
) (s *SubmitTxResponse, err error) {
	tx := SubmitTx{
		Cbor: data,
	}
	var (
		payload = makePayload(
			"submitTransaction",
			Map{"transaction": tx},
			Map{},
		)
		raw json.RawMessage
	)
	if err := c.query(ctx, payload, &raw); err != nil {
		return nil, fmt.Errorf("failed to submit TX: %w", err)
	}

	return readSubmitTx(raw)
}

func readSubmitTx(data []byte) (r *SubmitTxResponse, err error) {
	e, err1 := readSubmitTxError(data)
	id, err2 := readSubmitTxResult(data)
	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf(
			"could not parse submit tx response; neither error (%w) nor result (%w)",
			err1,
			err2,
		)
	}
	if err1 == nil {
		return &SubmitTxResponse{Error: e}, nil
	}
	if err2 == nil {
		return &SubmitTxResponse{ID: id}, nil
	}
	return nil, fmt.Errorf(
		"could not parse submit tx response: %s",
		string(data),
	)
}

type SubmitTxResponse struct {
	ID    string
	Error *SubmitTxError
}

type SubmitTxError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

func readSubmitTxError(data []byte) (*SubmitTxError, error) {
	value, _, _, err := jsonparser.Get(data, "error")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse SubmitTx error: %w %s",
			err,
			data,
		)
	}
	var e SubmitTxError
	if err := json.Unmarshal(value, &e); err != nil {
		return nil, fmt.Errorf(
			"failed to parse SubmitTx error: %w %s",
			err,
			data,
		)
	}
	return &e, nil
}

func readSubmitTxResult(data []byte) (string, error) {
	value, dataType, _, err := jsonparser.Get(data, "result")
	if err != nil {
		return "", fmt.Errorf(
			"failed to parse SubmitTx response: %w %s",
			err,
			string(data),
		)
	}

	switch dataType {
	case jsonparser.Object:
		var result struct {
			Transaction struct {
				ID string
			}
		}
		if err := json.Unmarshal(value, &result); err != nil {
			return "", fmt.Errorf("failed to parse SubmitTx response: %w", err)
		}
		return result.Transaction.ID, nil
	default:
		return "", fmt.Errorf("failed to parser SubmitTx response: %w", err)
	}
}

func (c *Client) SubmitTxV5(ctx context.Context, data string) (err error) {
	var (
		payload = makePayloadV5("SubmitTx", Map{"submit": data})
		raw     json.RawMessage
	)
	if err := c.query(ctx, payload, &raw); err != nil {
		return fmt.Errorf("failed to submit TX: %w", err)
	}
	return readSubmitTxV5(raw)
}

// SubmitTxError encapsulates the SubmitTx errors and allows the results to be parsed
type SubmitTxErrorV5 struct {
	messages []json.RawMessage
}

// HasErrorCode returns true if the error contains the provided code
func (s SubmitTxErrorV5) HasErrorCode(errorCode string) bool {
	errorCodes, _ := s.ErrorCodes()
	for _, ec := range errorCodes {
		if ec == errorCode {
			return true
		}
	}
	return false
}

// ErrorCodes the list of errors codes
func (s SubmitTxErrorV5) ErrorCodes() (keys []string, err error) {
	for _, data := range s.messages {
		if bytes.HasPrefix(data, []byte(`"`)) {
			var key string
			if err := json.Unmarshal(data, &key); err != nil {
				return nil, fmt.Errorf(
					"failed to decode string, %v",
					string(data),
				)
			}
			keys = append(keys, key)
			continue
		}

		var messages map[string]json.RawMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			return nil, fmt.Errorf("failed to decode object, %v", string(data))
		}

		for key := range messages {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// Messages returns the raw messages from SubmitTxErrorV5
func (s SubmitTxErrorV5) Messages() []json.RawMessage {
	return s.messages
}

// Error implements the error interface
func (s SubmitTxErrorV5) Error() string {
	keys, _ := s.ErrorCodes()
	return fmt.Sprintf("SubmitTx failed: %v", strings.Join(keys, ", "))
}

func readSubmitTxV5(data []byte) error {
	value, dataType, _, err := jsonparser.Get(data, "result", "SubmitFail")
	if err != nil {
		if errors.Is(err, jsonparser.KeyPathNotFoundError) {
			return nil
		}
		return fmt.Errorf("failed to parse SubmitTx response: %w", err)
	}

	switch dataType {
	case jsonparser.Array:
		var messages []json.RawMessage
		if err := json.Unmarshal(value, &messages); err != nil {
			return fmt.Errorf(
				"failed to parse SubmitTx response: array: %w",
				err,
			)
		}
		if len(messages) == 0 {
			return nil
		}
		return SubmitTxErrorV5{messages: messages}

	case jsonparser.Object:
		return SubmitTxErrorV5{messages: []json.RawMessage{value}}

	default:
		return fmt.Errorf("SubmitTx failed: %v", string(value))
	}
}
