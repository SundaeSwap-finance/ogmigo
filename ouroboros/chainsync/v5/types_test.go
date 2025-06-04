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

package v5

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync"
	"github.com/stretchr/testify/assert"
)

const TestDatumKey = 918273

func TestV5(t *testing.T) {
	t.Run("TxFromV6", func(t *testing.T) {
		rawData, err := os.ReadFile("test_data/Tx_v6.json")
		assert.Nil(t, err)

		var expectedV6 chainsync.Tx
		err = json.Unmarshal(rawData, &expectedV6)
		assert.Nil(t, err)
		v5Conversion := TxFromV6(expectedV6)
		v6Conversion := v5Conversion.ConvertToV6()

		for _, x := range v5Conversion.Witness.Bootstrap {
			var sig chainsync.Signature
			err = json.Unmarshal(x, &sig)
			assert.Nil(t, err)
		}

		network := "\"mainnet\""
		bootstrap := "{\"key\":\"d88f6028cc3d6d335115de3737bc2fe80a9a57a21a2c7c228ebc33b222e0897b\",\"signature\":\"/rRH7Ka4GfiLS2qsgalyABId1EUb/Mtl9z0x3ilrVALurUKEiAhjOtHUr7+tOi8ZZ85lUWrcpc03NnP3WKnAlg==\",\"chainCode\":\"12340000\",\"addressAttributes\":\"Lw==\"}"
		assert.Equal(t, "2f", expectedV6.Signatories[0].AddressAttributes)
		assert.Equal(t, "12340000", expectedV6.Signatories[0].ChainCode)
		assert.Equal(
			t,
			"d88f6028cc3d6d335115de3737bc2fe80a9a57a21a2c7c228ebc33b222e0897b",
			expectedV6.Signatories[0].Key,
		)
		assert.Equal(
			t,
			"feb447eca6b819f88b4b6aac81a97200121dd4451bfccb65f73d31de296b5402eead42848808633ad1d4afbfad3a2f1967ce65516adca5cd373673f758a9c096",
			expectedV6.Signatories[0].Signature,
		)
		assert.Equal(t, bootstrap, string(v5Conversion.Witness.Bootstrap[0]))
		assert.Equal(
			t,
			"IFb1lTq+ivhYQz6fAoPZQXuGgebeh5fIsM8rocK03mbss8yaUQpf871Qso2aAYaxjDadDHzMfUPRCJDpTyVxQg==",
			v5Conversion.Witness.Signatures["400019217786c3630fb121c455065b879055aa0ced5076a24abe8d6c837e0318"],
		)
		assert.Equal(t, json.RawMessage(network), v5Conversion.Body.Network)
		assert.Equal(t, json.RawMessage(network), v6Conversion.Network)
		assert.Equal(
			t,
			expectedV6.Signatories[0].AddressAttributes,
			v6Conversion.Signatories[2].AddressAttributes,
		)
		assert.Equal(
			t,
			expectedV6.Signatories[0].ChainCode,
			v6Conversion.Signatories[2].ChainCode,
		)
		assert.Equal(
			t,
			expectedV6.Signatories[0].Key,
			v6Conversion.Signatories[2].Key,
		)
		assert.Equal(
			t,
			expectedV6.Signatories[0].Signature,
			v6Conversion.Signatories[2].Signature,
		)
	})
}

func Test_ParseOgmiosMetadataV5(t *testing.T) {
	meta := json.RawMessage(`
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

	var o OgmiosAuxiliaryDataV5
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
	assert.Equal(t, 0, big.NewInt(123).Cmp(o.Body.Blob[TestDatumKey].IntField))
}

func Test_ParseOgmiosMetadataMapV5(t *testing.T) {
	meta := json.RawMessage(`
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

	var o OgmiosAuxiliaryDataV5
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
	assert.Equal(
		t,
		0,
		big.NewInt(1).Cmp(o.Body.Blob[TestDatumKey].MapField[0].Key.IntField),
	)
}

func Test_GetDatumBytesV5(t *testing.T) {
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
	datumBytes, err := GetMetadataDatumsV5(meta, TestDatumKey)
	assert.Nil(t, err)
	assert.Equal(t, expected, datumBytes[0])
}

func Test_UnmarshalOgmiosMetadataV5(t *testing.T) {
	meta := json.RawMessage(
		`{"674":{"map":[{"k":{"string":"msg"},"v":{"list":[{"string":"MuesliSwap Place Order"}]}}]},"1000":{"bytes":"01046034bf780d7e1a39a6ea628c54d70744664111947bfa319072b92d14f063133083b727c9f1b2e83c899982cc66da7aafd748e02206b849"},"1002":{"string":""},"1003":{"string":""},"1004":{"int":-949318},"1005":{"int":2650000},"1007":{"int":1},"1008":{"string":"547ceed647f57e64dc40a29b16be4f36b0d38b5aa3cd7afb286fc094"},"1009":{"string":"6262486f736b79"}}`,
	)
	var o OgmiosMetadataV5
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
}
