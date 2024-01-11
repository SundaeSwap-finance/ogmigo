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

package compatibility

import (
	"encoding/json"
	"fmt"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync"
	v5 "github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/v5"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// Support findIntersect (v6) / FindIntersection (v5) universally.
type CompatibleResultFindIntersection chainsync.ResultFindIntersectionPraos

// Deserialize either v5 or v6 values
func (c *CompatibleResultFindIntersection) UnmarshalJSON(data []byte) error {
	// Assume v6 responses first, then fall back to manual v5 processing.
	var r chainsync.ResultFindIntersectionPraos
	err := json.Unmarshal(data, &r)
	// We check intersection here, as that key is distinct from the other result types
	if err == nil && (r.Intersection != nil || r.Error != nil) {
		*c = CompatibleResultFindIntersection(r)
		return nil
	}

	var r5 v5.ResultFindIntersectionV5
	err = json.Unmarshal(data, &r5)
	if err == nil && (r5.IntersectionFound != nil || r5.IntersectionNotFound != nil) {
		*c = CompatibleResultFindIntersection(r5.ConvertToV6())
		return nil
	} else {
		return fmt.Errorf("unable to parse as either v5 or v6 FindIntersection: %w", err)
	}
}

// For now, serialize as v5
func (c CompatibleResultFindIntersection) MarshalJSON() ([]byte, error) {
	six := chainsync.ResultFindIntersectionPraos(c)
	var tip v5.PointStructV5
	if six.Tip != nil {
		tip = v5.PointStructV5{
			Hash: six.Tip.ID,
			Slot: six.Tip.Slot,
		}
		if six.Tip.Height != nil {
			tip.BlockNo = *six.Tip.Height
		}
	}
	var five v5.ResultFindIntersectionV5
	if six.Intersection != nil {
		five.IntersectionFound = &v5.IntersectionFoundV5{
			Point: v5.PointFromV6(*six.Intersection),
			Tip:   &tip,
		}
	} else {
		// TODO: tip is messy here; we would have to parse it out of the error data and that's awkward
		// It shouldn't be critical for now, so we just punt on that.
		five.IntersectionNotFound = &v5.IntersectionNotFoundV5{
			Tip: &tip,
		}
	}
	return json.Marshal(&five)
}

func (c *CompatibleResultFindIntersection) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	var s chainsync.ResultFindIntersectionPraos
	err := dynamodbattribute.Unmarshal(item, &s)
	if err == nil && s.Intersection != nil {
		*c = CompatibleResultFindIntersection(s)
		return nil
	}

	var v v5.ResultFindIntersectionV5
	err = dynamodbattribute.Unmarshal(item, &v)
	if err == nil && s.Intersection != nil {
		*c = CompatibleResultFindIntersection(v.ConvertToV6())
		return nil
	} else {
		return fmt.Errorf("unable to parse as either v5 or v6 FindIntersection: %w", err)
	}
}

func (c CompatibleResultFindIntersection) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	six := chainsync.ResultFindIntersectionPraos(c)
	five := v5.ResultFindIntersectionFromV6(six)
	av, err := dynamodbattribute.Marshal(&five)
	if err != nil {
		return err
	}
	*item = *av
	return nil
}

func (c CompatibleResultFindIntersection) String() string {
	return fmt.Sprintf("intersection=[%v] tip=[%v] error=[%v] id=[%v]", c.Intersection, c.Tip, c.Error, c.ID)
}

// Support nextBlock (v6) / RequestNext (v5) universally.
type CompatibleResultNextBlock chainsync.ResultNextBlockPraos

func (c *CompatibleResultNextBlock) UnmarshalJSON(data []byte) error {
	// Assume v6 responses first, then fall back to manual v5 processing.
	var r chainsync.ResultNextBlockPraos
	err := json.Unmarshal(data, &r)
	if err == nil && r.Direction != "" {
		*c = CompatibleResultNextBlock(r)
		return nil
	}

	var v v5.ResultNextBlockV5
	err = json.Unmarshal(data, &v)
	if err == nil && (v.RollBackward != nil || v.RollForward != nil) {
		*c = CompatibleResultNextBlock(v.ConvertToV6())
		return nil
	} else {
		return fmt.Errorf("unable to parse as either v5 of v6 NextBlock: %w", err)
	}
}

func (c CompatibleResultNextBlock) MarshalJSON() ([]byte, error) {
	six := chainsync.ResultNextBlockPraos(c)
	five := v5.ResultNextBlockFromV6(six)
	return json.Marshal(&five)
}

func (c *CompatibleResultNextBlock) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	var s chainsync.ResultNextBlockPraos
	err := dynamodbattribute.Unmarshal(item, &s)
	if err == nil && s.Direction != "" {
		*c = CompatibleResultNextBlock(s)
		return nil
	}

	var v v5.ResultNextBlockV5
	err = dynamodbattribute.Unmarshal(item, &v)
	if err == nil && (v.RollBackward != nil || v.RollForward != nil) {
		*c = CompatibleResultNextBlock(v.ConvertToV6())
		return nil
	} else {
		return fmt.Errorf("unable to parse as either v5 or v6 NextBlock: %w", err)
	}
}

func (c CompatibleResultNextBlock) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	six := chainsync.ResultNextBlockPraos(c)
	five := v5.ResultNextBlockFromV6(six)
	av, err := dynamodbattribute.Marshal(&five)
	if err != nil {
		return err
	}
	*item = *av
	return nil
}

func (c CompatibleResultNextBlock) String() string {
	return fmt.Sprintf("direction=[%v] tip=[%v] block=[%v] point=[%v]", c.Direction, c.Tip, c.Block, c.Point)
}

// Frontend for converting v5 JSON responses to v6.
type CompatibleResponsePraos chainsync.ResponsePraos

func (c *CompatibleResponsePraos) UnmarshalJSON(data []byte) error {
	var r chainsync.ResponsePraos
	err := json.Unmarshal(data, &r)
	if err == nil && r.Result != nil {
		*c = CompatibleResponsePraos(r)
		return nil
	}

	var r5 v5.ResponseV5
	err = json.Unmarshal(data, &r5)
	if err != nil {
		// Just skip all the data processing, as it's useless.
		return err
	}

	*c = CompatibleResponsePraos(r5.ConvertToV6())
	return nil
}

func (c CompatibleResponsePraos) MarshalJSON() ([]byte, error) {
	six := chainsync.ResponsePraos(c)
	return json.Marshal(v5.ResponseFromV6(six))
}

func (c *CompatibleResponsePraos) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	var s chainsync.ResponsePraos
	if err := dynamodbattribute.Unmarshal(item, &s); err != nil {
		var v v5.ResponseV5
		if err := dynamodbattribute.Unmarshal(item, &v); err != nil {
			return err
		}
		*c = CompatibleResponsePraos(v.ConvertToV6())
		return nil
	}
	*c = CompatibleResponsePraos(s)
	return nil
}

func (c CompatibleResponsePraos) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	six := chainsync.ResponsePraos(c)
	five := v5.ResponseFromV6(six)
	av, err := dynamodbattribute.Marshal(&five)
	if err != nil {
		return err
	}
	*item = *av
	return nil
}

func (r CompatibleResponsePraos) MustFindIntersectResult() CompatibleResultFindIntersection {
	if r.Method != chainsync.FindIntersectionMethod {
		panic(fmt.Errorf("must only use *Must* methods after switching on the findIntersection method; called on %v", r.Method))
	}
	t, ok := r.Result.(chainsync.ResultFindIntersectionPraos)
	if ok {
		return CompatibleResultFindIntersection(t)
	}
	u, ok := r.Result.(*chainsync.ResultFindIntersectionPraos)
	if ok && u != nil {
		return CompatibleResultFindIntersection(*u)
	}
	panic(fmt.Errorf("must method called on incompatible type"))
}

func (r CompatibleResponsePraos) MustNextBlockResult() CompatibleResultNextBlock {
	if r.Method != chainsync.NextBlockMethod {
		panic(fmt.Errorf("must only use *Must* methods after switching on the nextBlock method; called on %v", r.Method))
	}
	t, ok := r.Result.(chainsync.ResultNextBlockPraos)
	if ok {
		return CompatibleResultNextBlock(t)
	}
	u, ok := r.Result.(*chainsync.ResultNextBlockPraos)
	if ok && u != nil {
		return CompatibleResultNextBlock(*u)
	}
	panic(fmt.Errorf("must method called on incompatible type"))
}

type CompatibleValue shared.Value

func (c *CompatibleValue) UnmarshalJSON(data []byte) error {
	var v shared.Value
	err := json.Unmarshal(data, &v)
	if err == nil {
		*c = CompatibleValue(v)
		return nil
	}

	var r5 v5.ValueV5
	err = json.Unmarshal(data, &r5)
	if err != nil {
		return err
	}

	s := shared.Value{}
	if r5.Coins.BigInt().BitLen() != 0 {
		s.AddAsset(shared.CreateAdaCoin(r5.Coins))
	}
	for asset, coins := range r5.Assets {
		s.AddAsset(shared.Coin{AssetId: asset, Amount: coins})
	}
	*c = CompatibleValue(s)

	return nil
}

func (c CompatibleValue) MarshalJSON() ([]byte, error) {
	s := v5.ValueFromV6(shared.Value(c))
	return json.Marshal(&s)
}

func (c *CompatibleValue) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	var s shared.Value
	if err := dynamodbattribute.Unmarshal(item, &s); err != nil {
		var v v5.ValueV5
		if err := dynamodbattribute.Unmarshal(item, &v); err != nil {
			return err
		}
		*c = CompatibleValue(v.ConvertToV6())
		return nil
	}
	*c = CompatibleValue(s)
	return nil
}

func (c CompatibleValue) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	s := v5.ValueFromV6(shared.Value(c))
	av, err := dynamodbattribute.Marshal(&s)
	if err != nil {
		return err
	}
	*item = *av
	return nil
}

type CompatibleResult struct {
	NextBlock        *CompatibleResultNextBlock
	FindIntersection *CompatibleResultFindIntersection
}

func (c *CompatibleResult) UnmarshalJSON(data []byte) error {
	var rfi CompatibleResultFindIntersection
	err := json.Unmarshal(data, &rfi)
	r := CompatibleResult{}
	if err == nil {
		r.FindIntersection = &rfi
		*c = r
		return nil
	}

	var rnb CompatibleResultNextBlock
	err = json.Unmarshal(data, &rnb)
	if err == nil {
		r.NextBlock = &rnb
		*c = r
		return nil
	}

	return fmt.Errorf("unable to find an appropriate result")
}

func (c CompatibleResult) MarshalJSON() ([]byte, error) {
	if c.NextBlock != nil {
		return json.Marshal(c.NextBlock)
	}
	if c.FindIntersection != nil {
		return json.Marshal(c.FindIntersection)
	}
	return nil, fmt.Errorf("unable to marshal empty result")
}

func (c *CompatibleResult) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	var rfi CompatibleResultFindIntersection
	r := CompatibleResult{}
	if err := dynamodbattribute.Unmarshal(item, &rfi); err != nil {
		var rnb CompatibleResultNextBlock
		if err := dynamodbattribute.Unmarshal(item, &rnb); err != nil {
			return err
		}
		r.NextBlock = &rnb
		*c = r
		return nil
	}
	r.FindIntersection = &rfi
	*c = r
	return nil
}

func (c CompatibleResult) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	if c.NextBlock != nil {
		return c.NextBlock.MarshalDynamoDBAttributeValue(item)
	}
	if c.FindIntersection != nil {
		return c.FindIntersection.MarshalDynamoDBAttributeValue(item)
	}
	return fmt.Errorf("unable to marshal empty result")
}

// v5 and v6 transactions universally.
type CompatibleTx chainsync.Tx

// Deserialize either v5 or v6 values
func (c *CompatibleTx) UnmarshalJSON(data []byte) error {
	// Assume v6 responses first, then fall back to manual v5 processing.
	var tx chainsync.Tx
	err := json.Unmarshal(data, &tx)

	// We check spends here, as that key is distinct from the other result types.
	if err == nil && tx.Spends != "" {
		*c = CompatibleTx(tx)
		return nil
	}

	var txV5 v5.TxV5
	err = json.Unmarshal(data, &txV5)
	if err == nil && txV5.Raw != "" {
		*c = CompatibleTx(txV5.ConvertToV6())
		return nil
	} else {
		return fmt.Errorf("unable to parse as either v5 or v6 Tx: %w", err)
	}
}

// For now, serialize as v5
func (c CompatibleTx) MarshalJSON() ([]byte, error) {
	six := chainsync.Tx(c)
	five := v5.TxFromV6(six)
	return json.Marshal(&five)
}

func (c *CompatibleTx) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	var tx chainsync.Tx
	err := dynamodbattribute.Unmarshal(item, &tx)
	// We check spends here, as that key is distinct from the other result types.
	if err == nil && tx.Spends != "" {
		*c = CompatibleTx(tx)
		return nil
	}

	var txV5 v5.TxV5
	err = dynamodbattribute.Unmarshal(item, &txV5)
	if err == nil {
		*c = CompatibleTx(txV5.ConvertToV6())
		return nil
	} else {
		return fmt.Errorf("unable to parse as either v5 or v6 Tx: %w", err)
	}
}

func (c CompatibleTx) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	f := v5.TxFromV6(chainsync.Tx(c))

	av, err := dynamodbattribute.Marshal(&f)
	if err != nil {
		return err
	}
	*item = *av
	return nil
}
