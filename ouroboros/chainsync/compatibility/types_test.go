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
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	v5 "github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/v5"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/tj/assert"
)

const TestDatumKey = 918273

// TODO:
//   - break this up into smaller tests;
//   - put strings in a test_data folder;
//   - check the data we get to make sure it round tripped correctly
//   - test dynamodb marshalling
func TestCompatibleResult(t *testing.T) {
	t.Run("Roll Forward V5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/RollForward_v5.json")
		assert.Nil(t, err)

		var expected v5.ResultNextBlockV5
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultNextBlock
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)
		assert.Equal(
			t,
			"",
			compatible.Block.Transactions[0].Signatories[0].AddressAttributes,
		)
		assert.Equal(
			t,
			"909d",
			compatible.Block.Transactions[0].Signatories[0].ChainCode,
		)
		assert.Equal(
			t,
			"16d35a2dffb176e89d67ac7e7fca602f1deb5059f2edf48673108d72478f62e5",
			compatible.Block.Transactions[0].Signatories[0].Key,
		)
		assert.Equal(
			t,
			"e62e507db48986e427ccdda40b108df6dab84ce9a0f67377b92a71e5ff05db05aa6425a8ffdbbf5eef761e8c05f07d9fed8d2376f5c775123f55f595eb3fab66",
			compatible.Block.Transactions[0].Signatories[0].Signature,
		)
		assert.Equal(
			t,
			"fd",
			compatible.Block.Transactions[0].Signatories[1].AddressAttributes,
		)
		assert.Equal(
			t,
			"ac",
			compatible.Block.Transactions[0].Signatories[1].ChainCode,
		)
		assert.Equal(
			t,
			"7b5cb6d898502a692d256255efd9155053fa4ad247c3f74d152a67884e2c2424",
			compatible.Block.Transactions[0].Signatories[1].Key,
		)
		assert.Equal(
			t,
			"7650a7f156ce1be903249edf929f6d75bfc687f0613d33191d51bd4eb7ac32f41e189b9860dd49e76423abd3935dbb02ce9a52561426ffae7320149fafe35e65",
			compatible.Block.Transactions[0].Signatories[1].Signature,
		)
		assert.Equal(
			t,
			"aac4",
			compatible.Block.Transactions[0].Signatories[2].AddressAttributes,
		)
		assert.Equal(
			t,
			"7c",
			compatible.Block.Transactions[0].Signatories[2].ChainCode,
		)
		assert.Equal(
			t,
			"aa54eddec8234ca7a0205fd1262c0a7fc7e8ce56cb0edaba5ca2b66f58d8d8cf",
			compatible.Block.Transactions[0].Signatories[2].Key,
		)
		assert.Equal(
			t,
			"0b20ac60eac2a661b35ababd54d1722d0c2702535a53b5030a08f12be2cea1094175297261d9f9f1b1fced79edadc1470e4e13cc75d5f915722d6856fcaac6a9",
			compatible.Block.Transactions[0].Signatories[2].Signature,
		)
		assert.Equal(
			t,
			"d7ca16cc9549ae4f4fad53ab776a2dd2af25fa88de648d5ad7a18b106ab0bafc",
			compatible.Block.Transactions[0].Signatories[3].Key,
		)
		assert.Equal(
			t,
			"d6efd97e2a770eb79bb8ceb595920f6aab445b7a8b45ccf33f1f32016ca5f07620d27189ac93c275f22d7ed4521bd191f8166c59730081dab5eaa7d8e50ee0d0",
			compatible.Block.Transactions[0].Signatories[3].Signature,
		)
		assert.Equal(
			t,
			"d85d",
			compatible.Block.Transactions[1].Signatories[0].AddressAttributes,
		)
		assert.Equal(
			t,
			"",
			compatible.Block.Transactions[1].Signatories[0].ChainCode,
		)
		assert.Equal(
			t,
			"63176c55566a62e8bf7f7d93d7d890623fac8b46852501fe9317082619e7ee0a",
			compatible.Block.Transactions[1].Signatories[0].Key,
		)
		assert.Equal(
			t,
			"42e01143eadc791d481b04fe6c378692ec01b7e53bf45301b13886ac05d823f0672762c9d857b764fbcf19e7fbaa85db8bddf4642e7dbedefbb1e5e2b910578d",
			compatible.Block.Transactions[1].Signatories[0].Signature,
		)
		assert.Equal(
			t,
			"41b5",
			compatible.Block.Transactions[2].Signatories[0].AddressAttributes,
		)
		assert.Equal(
			t,
			"",
			compatible.Block.Transactions[2].Signatories[0].ChainCode,
		)
		assert.Equal(
			t,
			"440a92ff267209984b3c6afbee9c7d2390c91aa8ca92317e9113d6bba7ad8c4f",
			compatible.Block.Transactions[2].Signatories[0].Key,
		)
		assert.Equal(
			t,
			"5c48435bfa8699465a57ba2dfe23e1c28e17c5a2a23078458f96c8ab4e3484d2e71c58611dc7cb5004c08ff48ac3e3bae51314e13c6c8fdb4f23a53ac2fcb5f6",
			compatible.Block.Transactions[2].Signatories[0].Signature,
		)

		assert.EqualValues(
			t,
			expected.ConvertToV6(),
			chainsync.ResultNextBlockPraos(compatible),
		)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultNextBlockV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		// Ideally we'd like to check the full type round tripped, but for various reasons
		// this is super awkward, so we just check a few important properties
		assert.NotNil(t, got.RollForward)
		assert.NotNil(t, got.RollForward.Block.Babbage)
		assert.Equal(
			t,
			got.RollForward.Block.Babbage.HeaderHash,
			got.RollForward.Block.Babbage.HeaderHash,
		)
	})

	t.Run("Roll Backward V5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/RollBackward_v5.json")
		assert.Nil(t, err)

		var expected v5.ResultNextBlockV5
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultNextBlock
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected.ConvertToV6(), compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultNextBlockV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.RollBackward)
		assert.EqualValues(
			t,
			got.RollBackward.Point.String(),
			expected.RollBackward.Point.String(),
		)
	})

	t.Run("Roll Forward V6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/RollForward_v6.json")
		assert.Nil(t, err)

		var expected chainsync.ResultNextBlockPraos
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultNextBlock
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultNextBlockV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.RollForward)
		assert.NotNil(t, got.RollForward.Block.Allegra)
		assert.Equal(
			t,
			got.RollForward.Block.Allegra.HeaderHash,
			got.RollForward.Block.Allegra.HeaderHash,
		)
	})

	t.Run("Roll Backward V6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/RollBackward_v6.json")
		assert.Nil(t, err)

		var expected chainsync.ResultNextBlockPraos
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultNextBlock
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultNextBlockV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.RollBackward)
		assert.EqualValues(
			t,
			got.RollBackward.Point.ConvertToV6(),
			*expected.Point,
		)
	})

	t.Run("Intersection Found V5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/IntersectionFound_v5.json")
		assert.Nil(t, err)

		var expected v5.ResultFindIntersectionV5
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultFindIntersection
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected.ConvertToV6(), compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultFindIntersectionV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.IntersectionFound)
		assert.NotNil(t, got.IntersectionFound.Tip)
		assert.Equal(
			t,
			got.IntersectionFound.Tip.Hash,
			expected.IntersectionFound.Tip.Hash,
		)
	})

	t.Run("Intersection Not Found V5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/IntersectionNotFound_v5.json")
		assert.Nil(t, err)

		var expected v5.ResultFindIntersectionV5
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultFindIntersection
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected.ConvertToV6(), compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultFindIntersectionV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.IntersectionNotFound)
		assert.NotNil(t, got.IntersectionNotFound.Tip)
		assert.Equal(
			t,
			got.IntersectionNotFound.Tip.Hash,
			expected.IntersectionNotFound.Tip.Hash,
		)
	})

	t.Run("Intersection Found V6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/IntersectionFound_v6.json")
		assert.Nil(t, err)

		var expected chainsync.ResultFindIntersectionPraos
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultFindIntersection
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultFindIntersectionV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.IntersectionFound)
		assert.NotNil(t, got.IntersectionFound.Tip)
		assert.Equal(t, got.IntersectionFound.Tip.Hash, expected.Tip.ID)
	})

	t.Run("Intersection Not Found V6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/IntersectionNotFound_v6.json")
		assert.Nil(t, err)

		var expected chainsync.ResultFindIntersectionPraos
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResultFindIntersection
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResultFindIntersectionV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.NotNil(t, got.IntersectionNotFound)
	})

	t.Run("Real World Intersection Found", func(t *testing.T) {
		dataV5Result, err := os.ReadFile(
			"test_data/RealWorld_IntersectionFound_v5.json",
		)
		assert.Nil(t, err)
		var method9 CompatibleResult
		err = json.Unmarshal([]byte(dataV5Result), &method9)
		assert.Nil(t, err)
	})

	t.Run("Real World RollForward", func(t *testing.T) {
		dataV5Result, err := os.ReadFile(
			"test_data/RealWorld_RollForward_v5.json",
		)
		assert.Nil(t, err)
		var method10 CompatibleResult
		err = json.Unmarshal([]byte(dataV5Result), &method10)
		assert.Nil(t, err)
	})

	t.Run("Real World Taste Test", func(t *testing.T) {
		example, err := os.ReadFile(
			"test_data/RealWorld_TasteTest_RollForward_v5.json",
		)
		assert.Nil(t, err)
		var result CompatibleResult
		err = json.Unmarshal([]byte(example), &result)
		assert.Nil(t, err)
	})
}

func TestCompatibleResponse(t *testing.T) {
	t.Run("Full Response Roll Forward V5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Response_NextBlock_v5.json")
		assert.Nil(t, err)

		var expected v5.ResponseV5
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResponsePraos
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected.ConvertToV6(), compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResponseV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)
	})

	t.Run("Full Response Roll Forward V6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Response_NextBlock_v6.json")
		assert.Nil(t, err)

		var expected chainsync.ResponsePraos
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleResponsePraos
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ResponseV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)
	})
}

func TestDynamoDBMarshal(t *testing.T) {
	t.Run("Value v5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Value_v5.json")
		assert.Nil(t, err)

		var compatible CompatibleValue
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		av, err := dynamodbattribute.Marshal(&compatible)
		assert.Nil(t, err)
		assert.NotNil(t, av.M)

		var got v5.ValueV5
		err = dynamodbattribute.Unmarshal(av, &got)
		assert.Nil(t, err)
		assert.EqualValues(t, compatible, got.ConvertToV6())
	})
	t.Run("Value v6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Value_v6.json")
		assert.Nil(t, err)

		var compatible CompatibleValue
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		av, err := dynamodbattribute.Marshal(&compatible)
		assert.Nil(t, err)
		assert.NotNil(t, av.M)

		var got v5.ValueV5
		err = dynamodbattribute.Unmarshal(av, &got)
		assert.Nil(t, err)
		assert.EqualValues(t, compatible, got.ConvertToV6())
	})
	// If we need a compatible Tx type, uncomment this test
	t.Run("Tx v5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Tx_v5.json")
		assert.Nil(t, err)

		var compatible CompatibleTx
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		av, err := dynamodbattribute.Marshal(&compatible)
		assert.Nil(t, err)
		assert.NotNil(t, av.M)

		var got v5.TxV5
		err = dynamodbattribute.Unmarshal(av, &got)
		assert.Nil(t, err)
		assert.EqualValues(t, compatible, got.ConvertToV6())
	})
	t.Run("Tx v6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Tx_v6.json")
		assert.Nil(t, err)

		var compatible CompatibleTx
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		av, err := dynamodbattribute.Marshal(&compatible)
		assert.Nil(t, err)
		assert.NotNil(t, av.M)
	})
}

func TestSerializeCompatibleValue(t *testing.T) {
	t.Run("Value v5", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Value_v5.json")
		assert.Nil(t, err)

		var expected v5.ValueV5
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleValue
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected.ConvertToV6(), compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ValueV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, got)
	})
	t.Run("Value v6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Value_v6.json")
		assert.Nil(t, err)

		var expected shared.Value
		err = json.Unmarshal(rawData, &expected)
		assert.Nil(t, err)

		var compatible CompatibleValue
		err = json.Unmarshal(rawData, &compatible)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, compatible)

		bytes, err := json.Marshal(&compatible)
		assert.Nil(t, err)

		var got v5.ValueV5
		err = json.Unmarshal(bytes, &got)
		assert.Nil(t, err)

		assert.EqualValues(t, expected, got.ConvertToV6())
	})
}

func Test_ValueChecks(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		equal1 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(1000000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(1234567890),
			},
		)
		equal2 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(1000000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(1234567890),
			},
		)
		if !shared.Equal(equal1, equal2) {
			t.Fatalf("%v and %v are not equal", equal1, equal2)
		}

		val1 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(1000001)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(1234567890),
			},
		)
		val2 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(1000000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(1234567890),
			},
		)
		val3 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(1000000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(12345678900),
			},
		)
		val4 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(1000000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(1234567890),
			},
		)
		if !shared.GreaterThanOrEqual(val1, val2) {
			t.Fatalf("%v is not greater than %v", val1, val2)
		}
		if !shared.LessThanOrEqual(val2, val1) {
			t.Fatalf("%v is not less than %v", val1, val2)
		}
		if !shared.GreaterThanOrEqual(val3, val4) {
			t.Fatalf("%v is not greater than %v", val3, val4)
		}
		if !shared.LessThanOrEqual(val4, val3) {
			t.Fatalf("%v is not less than %v", val4, val3)
		}
		if ok, err := shared.Enough(val3, val4); !ok {
			t.Fatalf(
				"%v does not have enough assets for %v: %v",
				val4,
				val3,
				err.Error(),
			)
		}
		if shared.Equal(val1, val2) {
			t.Fatalf("%v and %v are equal", val1, val2)
		}
		if shared.Equal(val3, val4) {
			t.Fatalf("%v and %v are equal", val3, val4)
		}

		val5 := shared.Add(val1, val2)
		val6 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(2000001)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(2469135780),
			},
		)
		if !shared.Equal(val5, val6) {
			t.Fatalf("%v is not the expected value (%v)", val5, val6)
		}

		val7 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(600000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(2345678900),
			},
		)
		val8 := shared.Subtract(val3, val7)
		val9 := shared.ValueFromCoins(
			shared.CreateAdaCoin(num.Int64(400000)),
			shared.Coin{
				AssetId: shared.FromSeparate("abra", "cadabra"),
				Amount:  num.Int64(10000000000),
			},
		)
		if !shared.Equal(val8, val9) {
			t.Fatalf("%v is not the expected value (%v)", val8, val9)
		}
	})
}

func Test_ParseOgmiosMetadata(t *testing.T) {
	meta1 := json.RawMessage(`
          {
            "hash": "00",
            "body": {
              "blob": {
                "918273": {
                  "int": 123
                }
              }
            }
          }`,
	)

	var o1 CompatibleOgmiosAuxiliaryData
	err := json.Unmarshal(meta1, &o1)
	assert.Nil(t, err)
	labels1 := *(o1.Labels)
	assert.Equal(t, 0, big.NewInt(123).Cmp(labels1[TestDatumKey].Json.IntField))

	meta2 := json.RawMessage(`
          {
            "hash": "00",
            "labels": {
              "918273": {
                "json": {
                  "int": 123
                }
              }
            }
          }`,
	)

	var o2 CompatibleOgmiosAuxiliaryData
	err = json.Unmarshal(meta2, &o2)
	assert.Nil(t, err)
	labels2 := *(o2.Labels)
	assert.Equal(t, 0, big.NewInt(123).Cmp(labels2[TestDatumKey].Json.IntField))
}

func Test_ParseOgmiosMetadataMap(t *testing.T) {
	meta1 := json.RawMessage(`
          {
            "hash": "00",
            "body": {
              "blob": {
                "918273": {
                  "map": [
                    {
                      "k": { "int": 1 },
                      "v": { "string": "foo" }
                    },
                    {
                      "k": { "int": 2 },
                      "v": { "string": "bar" }
                    }
                  ]
                }
              }
            }
          }`,
	)

	var o1 CompatibleOgmiosAuxiliaryData
	err := json.Unmarshal(meta1, &o1)
	assert.Nil(t, err)
	labels1 := *(o1.Labels)
	assert.Equal(
		t,
		0,
		big.NewInt(1).Cmp(labels1[TestDatumKey].Json.MapField[0].Key.IntField),
	)

	meta2 := json.RawMessage(`
          {
            "hash": "00",
            "labels": {
              "918273": {
                "json": {
                  "map": [
                    {
                      "k": { "int": 1 },
                      "v": { "string": "foo" }
                    },
                    {
                      "k": { "int": 2 },
                      "v": { "string": "bar" }
                    }
                  ]
                }
              }
            }
          }`,
	)

	var o2 CompatibleOgmiosAuxiliaryData
	err = json.Unmarshal(meta2, &o2)
	assert.Nil(t, err)
	labels2 := *(o2.Labels)
	assert.Equal(
		t,
		0,
		big.NewInt(1).Cmp(labels2[TestDatumKey].Json.MapField[0].Key.IntField),
	)
}

func Test_GetDatumBytes(t *testing.T) {
	meta := json.RawMessage(`
          {
            "hash": "00",
            "body": {
              "blob": {
                "918273": {
                  "map": [
                    {
                      "k": {
                        "bytes": "5e60a2d4ebe669605f5b9cc95844122749fb655970af9ef30aad74f6abc7455e"
                      },
                      "v": {
                        "list":
                          [
                            {
                              "bytes": "d8799f4100d8799fd8799fd8799fd8799f581c694bc6017f9d74a5d9b3ef377b42b9fe4967a04fb1844959057f35bbffd87a80ffd87a80ffd8799f581c694bc6"
                            },
                            {
                              "bytes": "017f9d74a5d9b3ef377b42b9fe4967a04fb1844959057f35bbffff1a002625a0d87b9fd87a9fd8799f1a0007a1201a006312c3ffffffff"
                            }
                          ]
                      }
                    }
                  ]
                }
              }
            }
          }`,
	)
	bytes :=
		"d8799f4100d8799fd8799fd8799fd8799" +
			"f581c694bc6017f9d74a5d9b3ef377b42" +
			"b9fe4967a04fb1844959057f35bbffd87" +
			"a80ffd87a80ffd8799f581c694bc6017f" +
			"9d74a5d9b3ef377b42b9fe4967a04fb18" +
			"44959057f35bbffff1a002625a0d87b9f" +
			"d87a9fd8799f1a0007a1201a006312c3f" +
			"fffffff"
	expected, err := hex.DecodeString(bytes)
	assert.Nil(t, err)
	datumBytes, err := GetMetadataDatums(meta, TestDatumKey)
	assert.Nil(t, err)
	assert.Equal(t, expected, datumBytes[0])
}

func Test_UnmarshalTxWithNilMetadata(t *testing.T) {
	data, err := os.ReadFile("test_data/TxWithNilMetadata.json")
	assert.Nil(t, err)
	var tx CompatibleTx
	err = json.Unmarshal(data, &tx)
	assert.Nil(t, err)
	_, err = GetMetadataDatumMap(tx.Metadata, 103251)
	assert.Nil(t, err)
}
