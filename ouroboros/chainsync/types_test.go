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

package chainsync

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/fxamacker/cbor/v2"
	"github.com/nsf/jsondiff"
	"github.com/stretchr/testify/assert"
)

const TestDatumKey = 918273

func TestUnmarshal(t *testing.T) {
	err := filepath.Walk(
		"../../ext/ogmios/server/test/vectors/NextBlockResponse",
		assertStructMatchesSchema(t),
	)
	if err != nil {
		t.Fatalf("got %v; want nil", err)
	}
	decoder := json.NewDecoder(nil)
	decoder.DisallowUnknownFields()
}

func assertStructMatchesSchema(t *testing.T) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		path, _ = filepath.Abs(path)
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		//nolint:errcheck
		defer f.Close()

		decoder := json.NewDecoder(f)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&ResponsePraos{})
		if err != nil {
			t.Fatalf(
				"got %v; want nil: %v",
				err,
				fmt.Sprintf("struct did not match schema for file, %v", path),
			)
		}

		return nil
	}
}

func TestDynamodbSerialize(t *testing.T) {
	t.SkipNow()
	err := filepath.Walk(
		"../../ext/ogmios/server/test/vectors/NextBlockResponse",
		assertDynamoDBSerialize(t),
	)
	assert.Nil(t, err)
}

// TODO - This assumes non-Byron blocks. We're not technically supporting Byron in v6.
// Rework this test to ignore Byron blocks?
func assertDynamoDBSerialize(t *testing.T) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		t.Run(path, func(t *testing.T) {
			assert.Nil(t, err)
			if info.IsDir() {
				return
			}

			path, _ = filepath.Abs(path)
			f, err := os.Open(path)
			assert.Nil(t, err)
			//nolint:errcheck
			defer f.Close()

			var want ResponsePraos
			decoder := json.NewDecoder(f)
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&want)
			assert.Nil(t, err)

			item, err := dynamodbattribute.Marshal(want)
			assert.Nil(t, err)

			var got ResponsePraos
			err = dynamodbattribute.Unmarshal(item, &got)
			assert.Nil(t, err)

			w, err := json.Marshal(want)
			assert.Nil(t, err)

			g, err := json.Marshal(got)
			assert.Nil(t, err)

			opts := jsondiff.DefaultConsoleOptions()
			diff, s := jsondiff.Compare(w, g, &opts)

			if got, want := diff, jsondiff.FullMatch; !reflect.DeepEqual(
				got,
				want,
			) {
				fmt.Println(s)
				assert.EqualValues(t, got, want, "JSON Diff is not full match")
			}

		})
		return nil
	}
}

func TestPoint_CBOR(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		want := PointString("origin")
		item, err := cbor.Marshal(want.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		var point Point
		err = cbor.Unmarshal(item, &point)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if got, want := point.PointType(), PointTypeString; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}

		got, ok := point.PointString()
		if !ok {
			t.Fatalf("got false; want true")
		}
		if got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
	})

	t.Run("struct", func(t *testing.T) {
		h := uint64(123)
		want := &PointStruct{
			Height: &h,
			ID:     "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
			Slot:   456,
		}
		item, err := cbor.Marshal(want.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		var point Point
		err = cbor.Unmarshal(item, &point)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if got, want := point.PointType(), PointTypeStruct; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}

		got, ok := point.PointStruct()
		if !ok {
			t.Fatalf("got false; want true")
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v; want %#v", got, want)
		}
	})
}

func TestPoint_DynamoDB(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		want := PointString("origin")
		item, err := dynamodbattribute.Marshal(want.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		var point Point
		err = dynamodbattribute.Unmarshal(item, &point)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if got, want := point.PointType(), PointTypeString; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}

		got, ok := point.PointString()
		if !ok {
			t.Fatalf("got false; want true")
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
	})

	t.Run("struct", func(t *testing.T) {
		h := uint64(123)
		want := &PointStruct{
			Height: &h,
			ID:     "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
			Slot:   456,
		}
		item, err := dynamodbattribute.Marshal(want.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		var point Point
		err = dynamodbattribute.Unmarshal(item, &point)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if got, want := point.PointType(), PointTypeStruct; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}

		got, ok := point.PointStruct()
		if !ok {
			t.Fatalf("got false; want true")
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
	})
}

func TestPoint_JSON(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		want := PointString("origin")
		data, err := json.Marshal(want.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		var point Point
		err = json.Unmarshal(data, &point)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if got, want := point.PointType(), PointTypeString; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}

		got, ok := point.PointString()
		if !ok {
			t.Fatalf("got false; want true")
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
	})

	t.Run("struct", func(t *testing.T) {
		h := uint64(123)
		want := &PointStruct{
			Height: &h,
			ID:     "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
			Slot:   456,
		}
		data, err := json.Marshal(want.Point())
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		var point Point
		err = json.Unmarshal(data, &point)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if got, want := point.PointType(), PointTypeStruct; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("got %v; want %v", got, want)
		}

		got, ok := point.PointStruct()
		if !ok {
			t.Fatalf("got false; want true")
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
	})
}

func TestTxID_Index(t *testing.T) {
	if got, want := TxID("a#3").Index(), 3; got != want {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestTxID_TxHash(t *testing.T) {
	if got, want := TxID("a#3").TxHash(), "a"; got != want {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestPoints_Sort(t *testing.T) {
	s1 := PointString("1").Point()
	s2 := PointString("2").Point()
	p1 := PointStruct{Slot: 10}.Point()
	p2 := PointStruct{Slot: 10}.Point()
	tests := map[string]struct {
		Input Points
		Want  Points
	}{
		"string": {
			Input: Points{s1, s2},
			Want:  Points{s2, s1},
		},
		"points": {
			Input: Points{p1, p2},
			Want:  Points{p2, p1},
		},
		"mixed": {
			Input: Points{s1, p1, s2, p2},
			Want:  Points{p2, p1, s2, s1},
		},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			got := tc.Input
			sort.Sort(got)
			if !reflect.DeepEqual(got, tc.Want) {
				t.Fatalf("got %#v; want %#v", got, tc.Want)
			}
		})
	}
}

func TestPraosResponse(t *testing.T) {
	data := `{
		"jsonrpc": "2.0",
		"method": "nextBlock",
		"result": {
			"direction": "forward",
			"block": {
				"type": "praos",
				"era": "babbage",
				"id": "279050491668004eef2b6bd49e8c87c06a4b668aa9c59edbe5b61c9a5680b329",
				"size": {
					"bytes": 2
				},
				"height": 2,
				"slot": 2,
				"ancestor": "genesis",
				"issuer": {
					"verificationKey": "2c72a290211497ea824da75e9ed2a822d14e40dbe0f0d88a0df9aa43550933cc",
					"vrfVerificationKey": "07d6a01c8106ab634c412bb63e5f0beebe93853a3f1afa318b26c70f0c00afc7",
					"operationalCertificate": {
						"count": 4,
						"kes": {
							"period": 0,
							"verificationKey": "6c0bd2fc5909296acde44174d5969b5947cfc1ccd9974b73ffdc04549269b5e3"
						}
					},
					"leaderValue": {
						"output": "56e7e3a54ce74eb977e24a1063a153289e8c2073343c5e9ea8d800e9bb319a778e99062838324922ddcca94c1aee935a0442997327556862b94c7c17380aa24f",
						"proof": "b313b0b02a410b4560129b5d819c946d00761ba332c6601624cae810187e1adc529e6b079fbfba5981c91a9928687cf67bfa2a03359b967d6eb96c0baee46ecadf269ddca0683070b26e1279e0a11307"
					}
				},
				"protocol": {
					"version": {
						"major": 10,
						"minor": 0
					}
				},
				"transactions": [
					{
						"id": "9cd28711da282cb87cb9252e123f48c7b069619fc5f9d5bddeac0b11bbcf9d31",
						"spends": "collaterals",
						"inputs": [
							{
								"transaction": {
									"id": "602b75241874520aa7123f600be5780aafbf5112548de57724439fe8cd5e03b3"
								},
								"index": 3
							},
							{
								"transaction": {
									"id": "8ef29dc7180e0f34e472a73695cb1680d48b888e1123f9c2271145fa6d281a34"
								},
								"index": 4
							},
							{
								"transaction": {
									"id": "c4b98be24be62e04b587be7d6b4771a03f7663a04f6965b4d8a7f791bd9c695e"
								},
								"index": 5
							}
						],
						"references": [
							{
								"transaction": {
									"id": "22d10d66cdcc3ea3deaefa5b8fa2c3fe5fdfcbd4cf308a65d003ad2a93ee3179"
								},
								"index": 5
							},
							{
								"transaction": {
									"id": "ac7891786c12ad97f3a673eda87a07bbbaaec6c196161e29193e23b06b59294a"
								},
								"index": 0
							},
							{
								"transaction": {
									"id": "e17110fd84f7517d6fddca91ab151aefefa7f3c022a7499be040378358f5d94b"
								},
								"index": 1
							}
						],
						"outputs": [
							{
								"address": "addr_test1xz8kaamzwgl7qeqezvk28jc7xwqt96lymetwhpfpltlc9fyx5z9682dlu90yaaz8lygzge8tt0jnpwfsp7hj0vydp9tq7jw5p3",
								"value": {
									"ada": {
										"lovelace": 6599517526229999871
									},
									"4a1c412d8e2b3015a7fb7d382808fb7cb721bf93a56e8bb6661cdebe": {
										"a57b": 1
									}
								},
								"datum": "43e33bf3",
								"script": {
									"language": "native",
									"json": {
										"clause": "after",
										"slot": 7
									}
								}
							}
						],
						"collaterals": [
							{
								"transaction": {
									"id": "07b685fc880e2c84511dfe5cf7cb6d32f2f7b601b1bba96702c68d907f3705ce"
								},
								"index": 4
							},
							{
								"transaction": {
									"id": "21345a64d50aa371644b8f3d160021a1a7876295aaa6d58475d2c9b47e7cb428"
								},
								"index": 6
							},
							{
								"transaction": {
									"id": "a94f3da9edd689a255d187b676645fecbd863c62466fa07b50020673e9ddcd3a"
								},
								"index": 4
							},
							{
								"transaction": {
									"id": "d45b1cbe6d96f9215654db914cfe98c5e8e37e9548d89271e3b15903a9d1180d"
								},
								"index": 6
							}
						],
						"collateralReturn": {
							"address": "addr1y84cdp4x2n26uvlfe4txmnh0d37aunsf8evrlmwurpspw8zfh9mnrddv54u9lq8qpy09qcpupu2fnks5nwpknrm7vjps2vpr04",
							"value": {
								"ada": {
									"lovelace": 0
								},
								"2e12c5e499e0521b13837391beed1248a2e36117370662ee75918b56": {
									"35ab47a811413c": 4946671062515366198
								}
							},
							"datumHash": "c96ef3b6d4f5f5e1011391687a0c30bd5902342a257c945027af5524736e3996",
							"script": {
								"language": "native",
								"json": {
									"clause": "some",
									"atLeast": 0,
									"from": [
										{
											"clause": "signature",
											"from": "65fc709a5e019b8aba76f6977c1c8770e4b36fa76f434efc588747b7"
										}
									]
								}
							}
						},
						"totalCollateral": {
							"ada": {
								"lovelace": 927637
							}
						},
						"mint": {
							"be8dc1ca8b6b1735175a24b3b3c39d7327aba843c16a9b99924f3476": {
								"a3589dbc6194521a": 98371879850696154
							}
						},
						"network": "mainnet",
						"fee": {
							"ada": {
								"lovelace": 752644
							}
						},
						"validityInterval": {
							"invalidBefore": 7,
							"invalidAfter": 6
						},
						"proposals": [
							{
								"action": {
									"type": "treasuryWithdrawals",
									"withdrawals": {
										"64519ff082ace5007781306a885bf04a6dfc6df57fe486d3d98b7cb5": {
											"ada": {
												"lovelace": -659851
											}
										},
										"7e0531fb0ec714b6080d437377c91a56d3c4351e32ca2b27dd1f0195": {
											"ada": {
												"lovelace": 168719
											}
										},
										"9c444af5980051bfc60ef4a54068cae4ba543461c8bd37b0c174c9b7": {
											"ada": {
												"lovelace": -562643
											}
										}
									}
								}
							}
						],
						"signatories": [
							{
								"key": "8d0970408779a6266b19f32ef45905c6c8c17175a770280d72d9c0604077e5d3",
								"signature": "a29cbf419abab081a1dd52b6149bba970ccee39be117ba986cca980fb7820f9026a8f5ec37f7e9a3bc226520d2757763de1752618fb5f86a2f946d99537b6e91"
							},
							{
								"key": "0bc3ba44976cd03790946360224dab477229e0b5724e062d70eed7699c02e79c",
								"signature": "bd9eb1f85e591a6655dc8443b6ea89ea7a4114b48c80f42c2b8126f3182b0f498a8352638a5b69ce62ccc8230df373cd10d5f1780beeab8c8c9a6ca97be0011c",
								"chainCode": "b9",
								"addressAttributes": "9258ca"
							}
						],
						"scripts": {
							"4509cdddad21412c22c9164e10bc6071340ba235562f1575a35ded4d": {
								"language": "plutus:v1",
								"cbor": "450100002601"
							},
							"c370d10724c6b5a2448af41238e024ad470c0139da7f4b8527a47d74": {
								"language": "plutus:v1",
								"cbor": "46010000220011"
							}
						},
						"datums": {
							"2208e439244a1d0ef238352e3693098aba9de9dd0154f9056551636c8ed15dc1": "23"
						},
						"redeemers": [
							{
								"validator": {
									"index": 4,
									"purpose": "spend"
								},
								"redeemer": "a5239fd87d9f004023014273aeff9f00418b2143ee26a901ffffa1d87d9f43bafcf405ff425ccc029fd87c9f40ff446e943aa9d87e9f4375079e4318997905054197ffff23a3d87d9f43ca6d3e044233884206eb20ff0505a2024022019f43570e4322402324ff9f423456ff01d87c9fd87e9f01ff9f01ffff00",
								"executionUnits": {
									"memory": 5528516116957021378,
									"cpu": 3267967087510563235
								}
							},
							{
								"validator": {
									"index": 2,
									"purpose": "mint"
								},
								"redeemer": "d87b9f219f232041de009f44d47dc270ffffa3d87a9f0521ffa2024127054404aa1521d87980a24040444097f3a504d87d9f425ca843eb4ac704445f938c0905ff9f03ffa021ff",
								"executionUnits": {
									"memory": 5484661661513700435,
									"cpu": 3616344145952188136
								}
							},
							{
								"validator": {
									"index": 2,
									"purpose": "withdraw"
								},
								"redeemer": "d87c9f9f80a12442533fffa1d87d9f0201ffd8799f01ffff",
								"executionUnits": {
									"memory": 2494865883185442907,
									"cpu": 4035456766489310695
								}
							}
						]
					},
					{
						"id": "8dbba7a7bc4314310a39570be19390cdbb97c587aecd7ab9bfcb02d67ae68e90",
						"spends": "inputs",
						"inputs": [],
						"references": [
							{
								"transaction": {
									"id": "656b8f3001fadd1c09ac972c2d4c497ac23d3df79e5975d7ce0c9e0faa0254be"
								},
								"index": 5
							},
							{
								"transaction": {
									"id": "a45b136c6c4ba5a0a50e9eecbf8aa9b3ba76a51fb3e96e7db58e272055429016"
								},
								"index": 4
							},
							{
								"transaction": {
									"id": "c4c17a70f7dcf4426b8e09b13bab69899dd22e99212bdda2f5d4c3c3fb2d8468"
								},
								"index": 2
							},
							{
								"transaction": {
									"id": "f7533165414b0be3778d4ea32e85e2d69a1c2bae10bc03bcc72a85f3959bf653"
								},
								"index": 1
							}
						],
						"outputs": [
							{
								"address": "addr_test1xqzw54uwqf90cd4ujrnk4ll2xqp23ud5z8uf0ux8jdvrela8ntznpfc7svwwr0ak5jj3j060dw09nvtrhnr4l803puyscse43f",
								"value": {
									"ada": {
										"lovelace": 1570446258331790065
									},
									"d2a51a7e7678a02de266788af63481ebaa437626cc87b8bf85d25f25": {
										"36": 1
									}
								},
								"datumHash": "8c19ef57d6180c563bd5f54f354aebc865fb07ca98b2cb4c62e1983575bba82f",
								"script": {
									"language": "native",
									"json": {
										"clause": "some",
										"atLeast": 2,
										"from": [
											{
												"clause": "after",
												"slot": 14
											},
											{
												"clause": "before",
												"slot": 5
											},
											{
												"clause": "some",
												"atLeast": 1,
												"from": [
													{
														"clause": "some",
														"atLeast": 0,
														"from": []
													}
												]
											},
											{
												"clause": "any",
												"from": [
													{
														"clause": "all",
														"from": [
															{
																"clause": "signature",
																"from": "3542acb3a64d80c29302260d62c3b87a742ad14abf855ebc6733081e"
															}
														]
													},
													{
														"clause": "after",
														"slot": 6
													},
													{
														"clause": "all",
														"from": []
													}
												]
											}
										]
									}
								}
							},
							{
								"address": "addr1q9j6nk52759cv5dxxftfe4f2e3xqj0c4l7de0q5s6ecjqxdgxqe77rzpgudpjhjxms864zgxzmn4n4mqf60tdypqs8qssnn3cd",
								"value": {
									"ada": {
										"lovelace": 0
									},
									"2e12c5e499e0521b13837391beed1248a2e36117370662ee75918b56": {
										"36d0f791518dfc7bdaa008ac496a96aa51258194ff9dfe52482d0ded489884": 2
									}
								},
								"datumHash": "e60700a4bdc5696f3b9162bb5bd6ca0bd0ed3d94a34e63d1ccaff374929efce6"
							},
							{
								"address": "addr_test1yrq9ayusqmlj862fzupd0euqrgugmv3punx9ya6fhfmkuju898c4uwkye65pt7tya9l43z3hevhadu5dj46gddfpa8hq6mzgtd",
								"value": {
									"ada": {
										"lovelace": 0
									},
									"2db8410d969b6ad6b6969703c77ebf6c44061aa51c5d6ceba46557e2": {
										"504f4bba25991217": 3183844862761547728
									}
								},
								"datumHash": "0519e3b4094e19f636e21e8363561511f79f4522f45c4407798265e8814ea4e6",
								"script": {
									"language": "native",
									"json": {
										"clause": "signature",
										"from": "76e607db2a31c9a2c32761d2431a186a550cc321f79cd8d6a82b29b8"
									}
								}
							}
						],
						"collaterals": [
							{
								"transaction": {
									"id": "31e438353d69c20239d794cd6c6cd655ea70dd2d13c5d5a4d86f66d5334fd812"
								},
								"index": 4
							},
							{
								"transaction": {
									"id": "9c42ebb024d342a9e5d17532f9fe70adaba11c76f2b9b3cd40a00cccab5742b9"
								},
								"index": 4
							},
							{
								"transaction": {
									"id": "ac09c34fa3bee9c3ddac946616d33c19a86f64b898cbeec7fc14b48372724414"
								},
								"index": 7
							}
						],
						"certificates": [
							{
								"type": "stakePoolRetirement",
								"stakePool": {
									"id": "pool19gsk9qeah4hdeynmwy5nj599d37jcclk0hfj9dzhrq6lwdf26kn",
									"retirementEpoch": 8
								}
							},
							{
								"type": "genesisDelegation",
								"delegate": {
									"id": "571e042583907e06ba31d5f9e54603866e84733927341e23129a4f5a",
									"vrfVerificationKeyHash": "f577eed815c23dbb12f108a478b640631cab09a45bfb766597191bb0d73709af"
								},
								"issuer": {
									"id": "fd85ee08154b867f0f8c6ed7a30ae37b3f85709421ddfbad913ea5a8"
								}
							}
						],
						"withdrawals": {
							"stake_test1uqnd9um8wq9tkss02tmgkvz5t7zhcgcafkr8mujaelkdvgg06hp3t": {
								"ada": {
									"lovelace": 386198
								}
							}
						},
						"mint": {
							"9734f32ccaf9a788dbcddaea2b9fcf08609220b830d251307ece89dd": {
								"66388c3c645cbbb07040092cc72f701e788639b0723b3e": -2339162255260347769
							}
						},
						"requiredExtraSignatories": [
							"22d625efd631ca95d6b6161af2630061e5544e95c33eb951452becf2",
							"27e564b57a4591a602d1e8829b358775700f7ebcddafb8904cb95caa",
							"6b8e0878dc04959c1bc31ee9e377b2d9375207994046f34094b44b6d",
							"acbbeea632059a71648cd5a31425fbed2a09c9dc2255fae73afe2926"
						],
						"network": "testnet",
						"scriptIntegrityHash": "8041fd1e79eb03cc137e4ac04c353772ab29526a346a0814969c05eb8cdbb969",
						"fee": {
							"ada": {
								"lovelace": 552489
							}
						},
						"validityInterval": {
							"invalidBefore": 5
						},
						"proposals": [
							{
								"action": {
									"type": "treasuryTransfer",
									"source": "reserves",
									"target": "treasury",
									"value": {
										"ada": {
											"lovelace": 756310
										}
									}
								}
							},
							{
								"action": {
									"type": "treasuryTransfer",
									"source": "reserves",
									"target": "treasury",
									"value": {
										"ada": {
											"lovelace": 730347
										}
									}
								}
							}
						],
						"signatories": [
							{
								"key": "92a4d2787be093bd09f7c7e049bf70ac17454e5a0633533c53760a0288910b4c",
								"signature": "560601407217aa04a2564d5489aafc9eb390bb878776d4c4f638ab8428e6e53debbbe5bd7222807c98c0935b4479ab54c547623ac9b2b1039b8863960d6c2989"
							},
							{
								"key": "dcf4aed30b4c6c06019bba53a04225d68a601bc3ba815181be6bde042160e327",
								"signature": "e7faaa25db55e87400cabc4d792df294e6ef11c2e256e50bc61dc5d020a492f7b1bdffb18f227838964a3b81474f1402eb4449866365e5d50fb235e6df6c36ff"
							},
							{
								"key": "bdec98d1f571bb3d75497932bc5b7befeb1a2588e717d0edefe5608e93ff01fb",
								"signature": "415d707b1f2c75ce2bf4eda787e321f4bd83f60d7872fc078a212e733b81fb90b252141c880c407d3954f5ce55f19e23840f28c0de8212235609ebf156071d2c"
							},
							{
								"key": "e15011915798644426a9880c00174f8a32adec99b207b9983df933e9dcf55a6f",
								"signature": "bcdac345f53017be63b48d4e3cdea49cc33a3de2116c26bdc1680f9eaf303f32c4a7144f543c448e9efc07b64dfeee0f37cb32ffe6fccc32f9fac7b9d0d86899",
								"chainCode": "2f",
								"addressAttributes": "eb635f"
							}
						],
						"redeemers": [
							{
								"validator": {
									"index": 3,
									"purpose": "spend"
								},
								"redeemer": "44ecd6996f",
								"executionUnits": {
									"memory": 5012437254351683362,
									"cpu": 8543895173845936757
								}
							},
							{
								"validator": {
									"index": 6,
									"purpose": "mint"
								},
								"redeemer": "a3d87a9fa12201ff447a1bbfb842422da4437b5ab10242c05340a0a5224310eb56444ba4d7bc41872000054299ea2140234044f384dfd4427f58",
								"executionUnits": {
									"memory": 5139577675069485071,
									"cpu": 1480105547444861962
								}
							}
						]
					},
					{
						"id": "aec47db7562b8a301d44cc19940ea128718b6df4564975df07614523ae0d03ca",
						"spends": "collaterals",
						"inputs": [
							{
								"transaction": {
									"id": "6990c88473bf459b0e0b199e37694f65f00ceda84795d25cc83a0edb4d84328d"
								},
								"index": 0
							},
							{
								"transaction": {
									"id": "8b6813130937ce28471b90be0977487236a3e17414e7316c4860ff5beeb1644b"
								},
								"index": 0
							}
						],
						"references": [
							{
								"transaction": {
									"id": "1507dd415258452e3f028c8a9a144154d880ec20eaaa3b29168434b25729eef0"
								},
								"index": 6
							},
							{
								"transaction": {
									"id": "82a70de2e76bd6840967369e350dfaca6b1f85af54443f77783c83657473e010"
								},
								"index": 1
							},
							{
								"transaction": {
									"id": "8ca57230e6c0966229aaefe6ebe83636893d264fd8a4670f10cdb3acd0b71c2a"
								},
								"index": 5
							},
							{
								"transaction": {
									"id": "a9c1db8c66e10208222749842c960117027f15381038415ebe43cff5e2d455f0"
								},
								"index": 5
							}
						],
						"outputs": [],
						"collaterals": [
							{
								"transaction": {
									"id": "87210589b5167dfb2ffc4d0633d31ebc7fb755575269411f2be595ef88e67b64"
								},
								"index": 5
							},
							{
								"transaction": {
									"id": "956238d42d055e1857cca5e81274e4269e4298d808eeacd342f2b4e3f1deb396"
								},
								"index": 8
							},
							{
								"transaction": {
									"id": "eb69f29e33aa4e29f48c48987e0268487e846911fd26a1c83a2c81a8a197249a"
								},
								"index": 3
							}
						],
						"collateralReturn": {
							"address": "addr1v9s8auaf7xrwcknzzg9ftysuhwe87qw8xg2gmcwd4l3v6dqg6ajma",
							"value": {
								"ada": {
									"lovelace": 0
								},
								"65d910ddf94a9322d06c0719cb9fcb541d6a2ddf44d83071bcfcdbb0": {
									"d93c48b6d1baa662b59e5fb646c2ddafa15afbef5abfbe65d60e1711": 2653892454332976533
								}
							},
							"script": {
								"language": "native",
								"json": {
									"clause": "before",
									"slot": 5
								}
							}
						},
						"certificates": [
							{
								"type": "stakeDelegation",
								"credential": "800fabbca2f56cb7a428a8066e3d8354ebc5bb882179924fa94dbad1",
								"stakePool": {
									"id": "pool128dhwtz47afh3v3afvdzpy07t0jl6yp3qws29v0lkryhzhkhj0n"
								}
							}
						],
						"withdrawals": {
							"stake17xd7s38syqung8dqh2eu9erwcejda2y0njle0tt880ljunq6glahd": {
								"ada": {
									"lovelace": 893298
								}
							},
							"stake1uxjy7wsp5ct2kjcpv7sec9mv6zm24mgyu4ls0rlj9rlp0wsvwx7xg": {
								"ada": {
									"lovelace": 367880
								}
							}
						},
						"mint": {
							"bada8f15a81da088c0aba36b314376ca3a55dec62fee84bf60e1d18a": {
								"22f398c8486cdc5e19a389": 6470390966211355955
							}
						},
						"requiredExtraScripts": [
							"24935256c338acf58307f54aa0f8d93443774e3fe2df6474c2330514"
						],
						"network": "mainnet",
						"fee": {
							"ada": {
								"lovelace": 276885
							}
						},
						"validityInterval": {
							"invalidBefore": 6
						},
						"metadata": {
							"hash": "5de7c98c16812894f26804d889e1223812f92362a6e6bebef61e0e3602220a2b",
							"labels": {
								"1": {
									"json": -4
								},
								"6": {
									"cbor": "40"
								}
							}
						},
						"signatories": [
							{
								"key": "933469cfcac0cc66a225810a9dacd90af017e2591f30d53ab0008c7c2f944a1c",
								"signature": "6f0a87eb505d838c24334d57c2ff1bff0d1943c8ba0c9a49fd7787d8b8c3f17963279236d9823666e4f069b611876e95f79396cd90577ea12d2fff0b697c1c0d"
							},
							{
								"key": "60b22ec1e7468b421fbb98253a2adce943fe24b6af8c12ad91dca44405a75d27",
								"signature": "787d6cb0ba1e08487617a7a8300f34563aae4e3e7a3e59cfd2eb135dbe3893e239d8a1311272b38e26761b165f27b2e353d7dca4d08dffeebc7d6b4ef6d7b11e"
							},
							{
								"key": "5ae259fe8086a6070fb4c1c88070d3dfc5669755944b7f0c7ddcabde38863118",
								"signature": "193c609fc14f1666e16b3aedf555c3d22848591ee8d2301ec19023e642582d666e6a31107be14882eeb693c1723acdc53596592777a44c7e2782f206eb13d4c5",
								"chainCode": "06b5c1",
								"addressAttributes": "644f"
							},
							{
								"key": "9b0602ae64a8314053e0265e0e85a7ad20ff9f91c11a571f0392cdff815e352b",
								"signature": "03ce3042628394bcdaeda5607efcbcfb363318c1d8cc68823ae17ee5bffcbfd67e313eb5e1dd5e77d63b9b3c28dbab85eacfafc1a4b06f6e4f66b84865c3606d",
								"addressAttributes": "4fd7e8"
							},
							{
								"key": "6217b243745683b6dc14472beeb932b37ae8491a0f71b8ecf0babdeda6d32e06",
								"signature": "4e8bf99d133ba3b56683c9397ab567d9d64a934cd084c830a4ef3cae532816cfa5860ef51b165bfede96561a46e63b5fcada05756afaaf5796ed14e70d94d8c1",
								"addressAttributes": "b5"
							}
						],
						"scripts": {
							"24935256c338acf58307f54aa0f8d93443774e3fe2df6474c2330514": {
								"language": "native",
								"json": {
									"clause": "before",
									"slot": 7
								}
							},
							"c93062738b164d0fc43c651aefe6cd075920a3ef410efdfc2ee43087": {
								"language": "native",
								"json": {
									"clause": "after",
									"slot": 4
								}
							},
							"fbcd782100e8ed24576ad871af30fbba34a810d97e8e0059f2bf5332": {
								"language": "native",
								"json": {
									"clause": "some",
									"atLeast": 0,
									"from": [
										{
											"clause": "any",
											"from": [
												{
													"clause": "after",
													"slot": 10
												},
												{
													"clause": "after",
													"slot": 5
												}
											]
										},
										{
											"clause": "some",
											"atLeast": 0,
											"from": []
										},
										{
											"clause": "any",
											"from": [
												{
													"clause": "any",
													"from": [
														{
															"clause": "signature",
															"from": "58e1b65718531b42494610c506cef10ff031fa817a8ff75c0ab180e7"
														},
														{
															"clause": "signature",
															"from": "a646474b8f5431261506b6c273d307c7569a4eb6c96b42dd4a29520a"
														},
														{
															"clause": "signature",
															"from": "b5ae663aaea8e500157bdf4baafd6f5ba0ce5759f7cd4101fc132f54"
														}
													]
												},
												{
													"clause": "signature",
													"from": "76e607db2a31c9a2c32761d2431a186a550cc321f79cd8d6a82b29b8"
												},
												{
													"clause": "all",
													"from": []
												},
												{
													"clause": "before",
													"slot": 15
												},
												{
													"clause": "signature",
													"from": "3542acb3a64d80c29302260d62c3b87a742ad14abf855ebc6733081e"
												}
											]
										}
									]
								}
							}
						},
						"datums": {
							"704f836b6b652f423d42ce3232f5e82875172fc7e00f8bc27d84c2445a3a0341": "a2039f9f42640922ff9f423531230144c4e8777003ff22a2404448eb4b9a44157dad2d249f425b5e40ffff41a2d87c9fd87d80ff"
						},
						"redeemers": [
							{
								"validator": {
									"index": 6,
									"purpose": "mint"
								},
								"redeemer": "d87c80",
								"executionUnits": {
									"memory": 8801224258524349976,
									"cpu": 6616915720093090519
								}
							},
							{
								"validator": {
									"index": 4,
									"purpose": "publish"
								},
								"redeemer": "d87a80",
								"executionUnits": {
									"memory": 6358196380493345925,
									"cpu": 8390607135841051777
								}
							}
						]
					},
					{
						"id": "811794abd0c2dedca0612900a9b266e23b5fcd75e001e7856206f2bcaf220c88",
						"spends": "collaterals",
						"inputs": [
							{
								"transaction": {
									"id": "3843f3602616324b01c8fde44c65b4ac2f989936d62bc201fc1c11cba3493389"
								},
								"index": 0
							},
							{
								"transaction": {
									"id": "4a91adf891738ed4a02f591c12c80ba92b3cc460ad3444f051a3114eb75d8715"
								},
								"index": 2
							},
							{
								"transaction": {
									"id": "920a67c9ece3cd33ef365201e463a3e75445160a73fbd4221469bf41fea4ce68"
								},
								"index": 6
							},
							{
								"transaction": {
									"id": "f21a6bb40ff5298dc3366ca1c5f6554eeadab709b4c68c786840e7188c7a11e5"
								},
								"index": 1
							}
						],
						"references": [
							{
								"transaction": {
									"id": "237f1830c447e23d3ac6631de7952d0ef04f57c1adb8702d004bb9cfbf6cfa24"
								},
								"index": 3
							}
						],
						"outputs": [
							{
								"address": "addr1z9rnpqh8akpysrf06r6hulwra7ppa8vrad293anjnd4stlgpej9pf4gc58c8tqskgxzjvapy5cfddf7ekvxlm9nwh4es0ppw0u",
								"value": {
									"ada": {
										"lovelace": 0
									},
									"4a1c412d8e2b3015a7fb7d382808fb7cb721bf93a56e8bb6661cdebe": {
										"4e30b6e8803fa5467064ca01ffff": 1
									}
								},
								"datumHash": "40985f20209c7e1882d7c241e596838ac574e6e25aef587f1820b37d8259a199",
								"script": {
									"language": "native",
									"json": {
										"clause": "after",
										"slot": 1
									}
								}
							}
						],
						"collaterals": [
							{
								"transaction": {
									"id": "063b2a635b367cb9f3ca833168df645aa5f1872da14c1424c8085ba818c3c9ab"
								},
								"index": 5
							},
							{
								"transaction": {
									"id": "17139b2b4cea718a74466e94a46b5c81d3b801a3354d5ac6e4569c9dcac41d0a"
								},
								"index": 7
							}
						],
						"collateralReturn": {
							"address": "2cWKMJemoBajSt8TNJTf1YchyTTE9BjSufoWMSenBPVx91obrA2GzC8JQKtwwu24vKXdd",
							"value": {
								"ada": {
									"lovelace": 2960440386428397936
								},
								"98977cab9ebd32f98e2752d205da6748005f9c253fee025fcb06457d": {
									"455d1f01": 1
								}
							},
							"datumHash": "52e84f75d61dbf5137d2d307ad2f058fb71ae1d2508bd5eb49b5e33b52caeffc",
							"script": {
								"language": "native",
								"json": {
									"clause": "signature",
									"from": "76e607db2a31c9a2c32761d2431a186a550cc321f79cd8d6a82b29b8"
								}
							}
						},
						"totalCollateral": {
							"ada": {
								"lovelace": 405778
							}
						},
						"mint": {
							"b0c53e2bf180858da4b64eb5598c5615bba7d723d2b604a83b7f9165": {
								"69aea618b291a97f4344b6c5a35b": 1196208561900056328
							}
						},
						"network": "mainnet",
						"scriptIntegrityHash": "51111abd93b1784d20d198f3d1c5743a172520ab6e095819f73a7dbf211e0d91",
						"fee": {
							"ada": {
								"lovelace": 356829
							}
						},
						"validityInterval": {
							"invalidBefore": 4,
							"invalidAfter": 5
						},
						"signatories": [
							{
								"key": "db8801f9d48d47186daa90d109539b14e112e5943aac9bfe1185f1e8ef0797c3",
								"signature": "3c1a792c27ab78a942aab9513283ef03cc8153eb5053df6e1a638c62b3085a9878526c795f10e56160361d668869ee7bd9f35508f4db481061dab5d3828bf8e3"
							},
							{
								"key": "6d72f4ec03ce0d09158a2919eeb77dfe221ea63e8f8272b01445aab5980496de",
								"signature": "a59c594aa470752d628d8227280eff758557218df6b4e50b22cc8de230cade6ae55b41201caee1fa17481de31e29a89c5bf29723830ec69eb2156a39ea1863e8",
								"addressAttributes": "d30720"
							},
							{
								"key": "4ddce0f6f96e2a283a5dc860f3c8e53b697f958c01df336df0ad2b46821591a2",
								"signature": "fb33b9e2ac005a7b81287dd46ca201f1c8dc826e7fe60b59cfa2d0a77edf4f561bf36abf647899477211630ec889120bbd22176768af3119f93e6422e1ec2241",
								"chainCode": "b5",
								"addressAttributes": "c498b4"
							},
							{
								"key": "cab4f05c5b2fbf95fa85a7792897959f19e83a506bac307638efce6b4c286a2b",
								"signature": "73e03b4a4bf6c7059f5f14d7d9f1692169fd11e9f4743c83628295b0abf1f2d777ea509953917750436fb4600a3e51ae5cf9469ac032a961373843c1a45fea5b",
								"addressAttributes": "df920d"
							}
						],
						"scripts": {
							"7789659c6184299a248e40ed68e6329d09f3839c556f546c42f042d6": {
								"language": "native",
								"json": {
									"clause": "after",
									"slot": 5
								}
							}
						},
						"datums": {
							"368f9390a7c19125f0e73cd5e26d40f7de9c43aa39bc6f83ccdccaed5a94cbb1": "a2a441b6a30141904206b321443ed7069405442164818343e97fc905d87b8080d87a9f0444875c79bb40ffa39f42dbef03ff9f41732124ff00a2040144c2ae37fb014196d87d9f44fac271c8ff01a203d87e9f0542dee84024ff9f43e60f0641bb44ecc3ee7000421b1dff9f000421ff"
						},
						"redeemers": [
							{
								"validator": {
									"index": 6,
									"purpose": "spend"
								},
								"redeemer": "02",
								"executionUnits": {
									"memory": 8173834249442977306,
									"cpu": 4076336471699615579
								}
							},
							{
								"validator": {
									"index": 3,
									"purpose": "mint"
								},
								"redeemer": "9f2440ff",
								"executionUnits": {
									"memory": 2815464405602228167,
									"cpu": 9135073017538346060
								}
							},
							{
								"validator": {
									"index": 7,
									"purpose": "mint"
								},
								"redeemer": "42ba47",
								"executionUnits": {
									"memory": 7350280539456537937,
									"cpu": 474227869721943704
								}
							}
						]
					}
				]
			},
			"tip": {
				"slot": 94381,
				"id": "b7b0b3aad5dd2a2eea209a2c5c1dc1be3d5d9f0ba48a9bb04e867535e018d68b",
				"height": 8207255
			}
		},
		"id": null
	}`

	var response ResponsePraos
	err := json.Unmarshal([]byte(data), &response)
	if err != nil {
		t.Fatalf("error unmarshalling response: %v", err)
	}
}

func TestVasil_DatumParsing_Base64(t *testing.T) {
	data := `{"datums": {"a": "2HmfWBzIboNaGwk6qBYQ/Tk19GPOUpkpze2Ldfe1HOZEQpwK/w=="}}`
	var response Witness
	err := json.Unmarshal([]byte(data), &response)
	if err != nil {
		t.Fatalf("error unmarshalling response: %v", err)
	}

	datumHex := response.Datums["a"]
	_, err = hex.DecodeString(datumHex)
	if err != nil {
		t.Fatalf("error decoding hex string: %v", err)
	}
}

func TestVasil_DatumParsing_Hex(t *testing.T) {
	data := `{"datums": {"a": "d8799f581cc86e835a1b093aa81610fd3935f463ce529929cded8b75f7b51ce644429c0aff"}}`
	var response Witness
	err := json.Unmarshal([]byte(data), &response)
	if err != nil {
		t.Fatalf("error unmarshalling response: %v", err)
	}

	datumHex := response.Datums["a"]
	_, err = hex.DecodeString(datumHex)
	if err != nil {
		t.Fatalf("error decoding hex string: %v", err)
	}
}

func TestVasil_BackwardsCompatibleWithExistingDynamoDB(t *testing.T) {
	data, err := os.ReadFile("testdata/scoop.json")
	assert.Nil(t, err)

	var item map[string]*dynamodb.AttributeValue
	err = json.Unmarshal(data, &item)
	assert.NoError(t, err)

	var response Tx
	err = dynamodbattribute.Unmarshal(item["tx"], &response)
	assert.NoError(t, err)
	fmt.Println(response.Datums)
}

func Test_ParseOgmiosMetadatum(t *testing.T) {
	meta := json.RawMessage(`{ "int": 123 }`)

	var o OgmiosMetadatum
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
	assert.Equal(t, OgmiosMetadatumTagInt, o.Tag)
	assert.Equal(t, 0, big.NewInt(123).Cmp(o.IntField))
}

func Test_ParseOgmiosMetadataV6(t *testing.T) {
	meta := json.RawMessage(`
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

	var o OgmiosAuxiliaryDataV6
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
	labels := *(o.Labels)
	assert.Equal(t, 0, big.NewInt(123).Cmp(labels[TestDatumKey].Json.IntField))
}

func Test_ParseOgmiosMetadataMapV6(t *testing.T) {
	meta := json.RawMessage(`
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

	var o OgmiosAuxiliaryDataV6
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
	labels := *(o.Labels)
	assert.Equal(
		t,
		0,
		big.NewInt(1).Cmp(labels[TestDatumKey].Json.MapField[0].Key.IntField),
	)
}

func Test_GetZapDatumBytesV6(t *testing.T) {
	meta := json.RawMessage(`
          {
            "hash": "00",
            "labels": {
              "918273": {
                "json": {
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
	datumBytes, err := GetMetadataDatumsV6(meta, TestDatumKey)
	assert.Nil(t, err)
	assert.Equal(t, expected, datumBytes[0])
}

func Test_UnmarshalOgmiosMetadataV6(t *testing.T) {
	meta := json.RawMessage(
		`{"674":{"map":[{"k":{"string":"msg"},"v":{"list":[{"string":"MuesliSwap Place Order"}]}}]},"1000":{"bytes":"01046034bf780d7e1a39a6ea628c54d70744664111947bfa319072b92d14f063133083b727c9f1b2e83c899982cc66da7aafd748e02206b849"},"1002":{"string":""},"1003":{"string":""},"1004":{"int":-949318},"1005":{"int":2650000},"1007":{"int":1},"1008":{"string":"547ceed647f57e64dc40a29b16be4f36b0d38b5aa3cd7afb286fc094"},"1009":{"string":"6262486f736b79"}}`,
	)
	var o OgmiosAuxiliaryDataLabelsV6
	err := json.Unmarshal(meta, &o)
	assert.Nil(t, err)
}
