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
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/fxamacker/cbor/v2"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync/num"
)

var (
	encOptions = cbor.CoreDetEncOptions()
	bNil       = []byte("nil")
)

type AssetID string

func (a AssetID) HasPolicyID(s string) bool {
	return len(s) == 56 && strings.HasPrefix(string(a), s)
}

func (a AssetID) HasAssetID(re *regexp.Regexp) bool {
	return re.MatchString(string(a))
}

func (a AssetID) IsZero() bool {
	return a == ""
}

func (a AssetID) MatchAssetName(re *regexp.Regexp) ([]string, bool) {
	if assetName := a.AssetName(); len(assetName) > 0 {
		ss := re.FindStringSubmatch(assetName)
		return ss, len(ss) > 0
	}
	return nil, false
}

func (a AssetID) String() string {
	return string(a)
}

func (a AssetID) AssetName() string {
	s := string(a)
	if index := strings.Index(s, "."); index > 0 {
		return s[index+1:]
	}
	return ""
}

func (a AssetID) AssetNameUTF8() (string, bool) {
	if data, err := hex.DecodeString(a.AssetName()); err == nil {
		if utf8.Valid(data) {
			return string(data), true
		}
	}
	return "", false
}

func (a AssetID) PolicyID() string {
	s := string(a)
	if index := strings.Index(s, "."); index > 0 {
		return s[:index]
	}
	return s // Assets with empty-string name come back as just the policy ID
}

type Block struct {
	Body       []Tx        `json:"body,omitempty"       dynamodbav:"body,omitempty"`
	Header     BlockHeader `json:"header,omitempty"     dynamodbav:"header,omitempty"`
	HeaderHash string      `json:"headerHash,omitempty" dynamodbav:"headerHash,omitempty"`
}

type BlockHeader struct {
	BlockHash       string                 `json:"blockHash,omitempty"       dynamodbav:"blockHash,omitempty"`
	BlockHeight     uint64                 `json:"blockHeight,omitempty"     dynamodbav:"blockHeight,omitempty"`
	BlockSize       uint64                 `json:"blockSize,omitempty"       dynamodbav:"blockSize,omitempty"`
	IssuerVK        string                 `json:"issuerVK,omitempty"        dynamodbav:"issuerVK,omitempty"`
	IssuerVrf       string                 `json:"issuerVrf,omitempty"       dynamodbav:"issuerVrf,omitempty"`
	LeaderValue     map[string][]byte      `json:"leaderValue,omitempty"     dynamodbav:"leaderValue,omitempty"`
	Nonce           map[string]string      `json:"nonce,omitempty"           dynamodbav:"nonce,omitempty"`
	OpCert          map[string]interface{} `json:"opCert,omitempty"          dynamodbav:"opCert,omitempty"`
	PrevHash        string                 `json:"prevHash,omitempty"        dynamodbav:"prevHash,omitempty"`
	ProtocolVersion map[string]int         `json:"protocolVersion,omitempty" dynamodbav:"protocolVersion,omitempty"`
	Signature       string                 `json:"signature,omitempty"       dynamodbav:"signature,omitempty"`
	Slot            uint64                 `json:"slot,omitempty"            dynamodbav:"slot,omitempty"`
}

type IntersectionFound struct {
	Point Point
	Tip   Point
}

type IntersectionNotFound struct {
	Tip Point
}

type PointType int

const (
	PointTypeString PointType = 1
	PointTypeStruct PointType = 2
)

var Origin = PointString("origin").Point()

type PointString string

func (p PointString) Point() Point {
	return Point{
		pointType:   PointTypeString,
		pointString: p,
	}
}

type PointStruct struct {
	BlockNo uint64 `json:"blockNo,omitempty" dynamodbav:"blockNo,omitempty"`
	Hash    string `json:"hash,omitempty"    dynamodbav:"hash,omitempty"`
	Slot    uint64 `json:"slot,omitempty"    dynamodbav:"slot,omitempty"`
}

func (p PointStruct) Point() Point {
	return Point{
		pointType:   PointTypeStruct,
		pointStruct: &p,
	}
}

type Point struct {
	pointType   PointType
	pointString PointString
	pointStruct *PointStruct
}

func (p Point) String() string {
	switch p.pointType {
	case PointTypeString:
		return string(p.pointString)
	case PointTypeStruct:
		if p.pointStruct.BlockNo == 0 {
			return fmt.Sprintf("slot=%v hash=%v", p.pointStruct.Slot, p.pointStruct.Hash)
		}
		return fmt.Sprintf("slot=%v hash=%v block=%v", p.pointStruct.Slot, p.pointStruct.Hash, p.pointStruct.BlockNo)
	default:
		return "invalid point"
	}
}

type Points []Point

func (pp Points) String() string {
	var ss []string
	for _, p := range pp {
		ss = append(ss, p.String())
	}
	return strings.Join(ss, ", ")
}

func (pp Points) Len() int      { return len(pp) }
func (pp Points) Swap(i, j int) { pp[i], pp[j] = pp[j], pp[i] }
func (pp Points) Less(i, j int) bool {
	pi, pj := pp[i], pp[j]
	switch {
	case pi.pointType == PointTypeStruct && pj.pointType == PointTypeStruct:
		return pi.pointStruct.Slot > pj.pointStruct.Slot
	case pi.pointType == PointTypeStruct:
		return true
	case pj.pointType == PointTypeStruct:
		return false
	default:
		return pi.pointString > pj.pointString
	}
}

// pointCBOR provide simplified internal wrapper
type pointCBOR struct {
	String PointString  `cbor:"1,keyasint,omitempty"`
	Struct *PointStruct `cbor:"2,keyasint,omitempty"`
}

func (p Point) PointType() PointType             { return p.pointType }
func (p Point) PointString() (PointString, bool) { return p.pointString, p.pointString != "" }

func (p Point) PointStruct() (*PointStruct, bool) { return p.pointStruct, p.pointStruct != nil }

func (p Point) MarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	switch p.pointType {
	case PointTypeString:
		item.S = aws.String(string(p.pointString))
	case PointTypeStruct:
		m, err := dynamodbattribute.MarshalMap(p.pointStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal point struct: %w", err)
		}
		item.M = m
	default:
		return fmt.Errorf("unable to unmarshal Point: unknown type")
	}
	return nil
}

func (p Point) MarshalCBOR() ([]byte, error) {
	switch p.pointType {
	case PointTypeString, PointTypeStruct:
		v := pointCBOR{
			String: p.pointString,
			Struct: p.pointStruct,
		}
		return cbor.Marshal(v)
	default:
		return nil, fmt.Errorf("unable to unmarshal Point: unknown type")
	}
}

func (p Point) MarshalJSON() ([]byte, error) {
	switch p.pointType {
	case PointTypeString:
		return json.Marshal(p.pointString)
	case PointTypeStruct:
		return json.Marshal(p.pointStruct)
	default:
		return nil, fmt.Errorf("unable to unmarshal Point: unknown type")
	}
}

func (p *Point) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 || bytes.Equal(data, bNil) {
		return nil
	}

	var v pointCBOR
	if err := cbor.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal Point: %w", err)
	}

	point := Point{
		pointType:   PointTypeString,
		pointString: v.String,
		pointStruct: v.Struct,
	}
	if point.pointStruct != nil {
		point.pointType = PointTypeStruct
	}

	*p = point

	return nil
}

func (p *Point) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	switch {
	case item == nil:
		return nil
	case item.S != nil:
		*p = Point{
			pointType:   PointTypeString,
			pointString: PointString(aws.StringValue(item.S)),
		}
	case len(item.M) > 0:
		var point PointStruct
		if err := dynamodbattribute.UnmarshalMap(item.M, &point); err != nil {
			return fmt.Errorf("failed to unmarshal point struct: %w", err)
		}
		*p = Point{
			pointType:   PointTypeStruct,
			pointStruct: &point,
		}
	}
	return nil
}

func (p *Point) UnmarshalJSON(data []byte) error {
	switch {
	case data[0] == '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("failed to unmarshal Point, %v: %w", string(data), err)
		}

		*p = Point{
			pointType:   PointTypeString,
			pointString: PointString(s),
		}

	default:
		var ps PointStruct
		if err := json.Unmarshal(data, &ps); err != nil {
			return fmt.Errorf("failed to unmarshal Point, %v: %w", string(data), err)
		}

		*p = Point{
			pointType:   PointTypeStruct,
			pointStruct: &ps,
		}
	}

	return nil
}

type ProtocolVersion struct {
	Major uint32
	Minor uint32
	Patch uint32 `json:"patch,omitempty"`
}

type RollBackward struct {
	Point Point `json:"point,omitempty" dynamodbav:"point,omitempty"`
	Tip   Point `json:"tip,omitempty"   dynamodbav:"tip,omitempty"`
}

type RollForward struct {
	Block RollForwardBlock `json:"block,omitempty" dynamodbav:"block,omitempty"`
	Tip   Point            `json:"tip,omitempty"   dynamodbav:"tip,omitempty"`
}

type RollForwardBlock struct {
	Allegra *Block      `json:"allegra,omitempty" dynamodbav:"allegra,omitempty"`
	Alonzo  *Block      `json:"alonzo,omitempty"  dynamodbav:"alonzo,omitempty"`
	Babbage *Block      `json:"babbage,omitempty" dynamodbav:"babbage,omitempty"`
	Byron   *ByronBlock `json:"byron,omitempty"   dynamodbav:"byron,omitempty"`
	Mary    *Block      `json:"mary,omitempty"    dynamodbav:"mary,omitempty"`
	Shelley *Block      `json:"shelley,omitempty" dynamodbav:"shelley,omitempty"`
}

func (r RollForwardBlock) PointStruct() PointStruct {
	if byron := r.Byron; byron != nil {
		return PointStruct{
			BlockNo: byron.Header.BlockHeight,
			Hash:    byron.Hash,
			Slot:    byron.Header.Slot,
		}
	}

	var block *Block
	switch {
	case r.Allegra != nil:
		block = r.Allegra
	case r.Alonzo != nil:
		block = r.Alonzo
	case r.Mary != nil:
		block = r.Mary
	case r.Shelley != nil:
		block = r.Shelley
	case r.Babbage != nil:
		block = r.Babbage
	default:
		return PointStruct{}
	}

	return PointStruct{
		BlockNo: block.Header.BlockHeight,
		Hash:    block.HeaderHash,
		Slot:    block.Header.Slot,
	}
}

type Result struct {
	IntersectionFound    *IntersectionFound    `json:",omitempty" dynamodbav:",omitempty"`
	IntersectionNotFound *IntersectionNotFound `json:",omitempty" dynamodbav:",omitempty"`
	RollForward          *RollForward          `json:",omitempty" dynamodbav:",omitempty"`
	RollBackward         *RollBackward         `json:",omitempty" dynamodbav:",omitempty"`
}

type Response struct {
	Type        string          `json:"type,omitempty"        dynamodbav:"type,omitempty"`
	Version     string          `json:"version,omitempty"     dynamodbav:"version,omitempty"`
	ServiceName string          `json:"servicename,omitempty" dynamodbav:"servicename,omitempty"`
	MethodName  string          `json:"methodname,omitempty"  dynamodbav:"methodname,omitempty"`
	Result      *Result         `json:"result,omitempty"      dynamodbav:"result,omitempty"`
	Reflection  json.RawMessage `json:"reflection,omitempty"  dynamodbav:"reflection,omitempty"`
}

type Tx struct {
	ID          string          `json:"id,omitempty"       dynamodbav:"id,omitempty"`
	InputSource string          `json:"inputSource,omitempty"  dynamodbav:"inputSource,omitempty"`
	Body        TxBody          `json:"body,omitempty"     dynamodbav:"body,omitempty"`
	Witness     Witness         `json:"witness,omitempty"  dynamodbav:"witness,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty" dynamodbav:"metadata,omitempty"`
	// Raw serialized transaction, base64.
	Raw string `json:"raw,omitempty" dynamodbav:"raw,omitempty"`
}

type TxBody struct {
	Certificates            []json.RawMessage `json:"certificates,omitempty"            dynamodbav:"certificates,omitempty"`
	Collaterals             []TxIn            `json:"collaterals,omitempty"             dynamodbav:"collaterals,omitempty"`
	Fee                     num.Int           `json:"fee,omitempty"                     dynamodbav:"fee,omitempty"`
	Inputs                  []TxIn            `json:"inputs,omitempty"                  dynamodbav:"inputs,omitempty"`
	Mint                    *Value            `json:"mint,omitempty"                    dynamodbav:"mint,omitempty"`
	Network                 json.RawMessage   `json:"network,omitempty"                 dynamodbav:"network,omitempty"`
	Outputs                 TxOuts            `json:"outputs,omitempty"                 dynamodbav:"outputs,omitempty"`
	RequiredExtraSignatures []string          `json:"requiredExtraSignatures,omitempty" dynamodbav:"requiredExtraSignatures,omitempty"`
	ScriptIntegrityHash     string            `json:"scriptIntegrityHash,omitempty"     dynamodbav:"scriptIntegrityHash,omitempty"`
	TimeToLive              int64             `json:"timeToLive,omitempty"              dynamodbav:"timeToLive,omitempty"`
	Update                  json.RawMessage   `json:"update,omitempty"                  dynamodbav:"update,omitempty"`
	ValidityInterval        ValidityInterval  `json:"validityInterval"                  dynamodbav:"validityInterval,omitempty"`
	Withdrawals             map[string]int64  `json:"withdrawals,omitempty"             dynamodbav:"withdrawals,omitempty"`
	CollateralReturn        *TxOut            `json:"collateralReturn,omitempty"        dynamodbav:"collateralReturn,omitempty"`
	TotalCollateral         *int64            `json:"totalCollateral,omitempty"         dynamodbav:"totalCollateral,omitempty"`
	References              []TxIn            `json:"references,omitempty"              dynamodbav:"references,omitempty"`
}

type TxID string

func NewTxID(txHash string, index int) TxID {
	return TxID(txHash + "#" + strconv.Itoa(index))
}

func (t TxID) String() string {
	return string(t)
}

func (t TxID) Index() int {
	if index := strings.Index(string(t), "#"); index > 0 {
		if v, err := strconv.Atoi(string(t[index+1:])); err == nil {
			return v
		}
	}
	return -1
}

func (t TxID) TxHash() string {
	if index := strings.Index(string(t), "#"); index > 0 {
		return string(t[0:index])
	}
	return ""
}

type TxIn struct {
	TxHash string `json:"txId"  dynamodbav:"txId"`
	Index  int    `json:"index" dynamodbav:"index"`
}

func (t TxIn) String() string {
	return t.TxHash + "#" + strconv.Itoa(t.Index)
}

func (t TxIn) TxID() TxID {
	return NewTxID(t.TxHash, t.Index)
}

type TxOut struct {
	Address   string          `json:"address,omitempty"   dynamodbav:"address,omitempty"`
	Datum     string          `json:"datum,omitempty"     dynamodbav:"datum,omitempty"`
	DatumHash string          `json:"datumHash,omitempty" dynamodbav:"datumHash,omitempty"`
	Value     Value           `json:"value,omitempty"     dynamodbav:"value,omitempty"`
	Script    json.RawMessage `json:"script,omitempty"    dynamodbav:"script,omitempty"`
}

type TxOuts []TxOut

func (tt TxOuts) FindByAssetID(assetID AssetID) (TxOut, bool) {
	for _, t := range tt {
		for gotAssetID := range t.Value.Assets {
			if gotAssetID == assetID {
				return t, true
			}
		}
	}
	return TxOut{}, false
}

type Datums map[string]string

func (d *Datums) UnmarshalJSON(i []byte) error {
	if i == nil {
		return nil
	}

	var raw map[string]interface{}
	err := json.Unmarshal(i, &raw)
	if err != nil {
		return fmt.Errorf("unable to unmarshal as raw map: %w", err)
	}

	results := make(Datums, len(raw))
	// for backwards compatibility, since ogmios switched Datum values from []byte to hex string
	// this should be safe to remove after we upgrade all ogmios nodes to >= 5.5.0
	for k, v := range raw {
		s, isString := v.(string)
		if !isString {
			return fmt.Errorf("expecting string, got %v", v)
		}
		asHex := s
		// if it's base64 encoded, convert it to a hex string.
		if _, err := hex.DecodeString(s); err != nil {
			rawDatum, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("unable to decode string %v: %w", s, err)
			}
			asHex = hex.EncodeToString(rawDatum)
		}
		results[k] = asHex
	}

	*d = results
	return nil
}

func (d *Datums) UnmarshalDynamoDBAttributeValue(item *dynamodb.AttributeValue) error {
	if item == nil {
		return nil
	}

	var raw map[string]interface{}
	if err := dynamodbattribute.UnmarshalMap(item.M, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal map: %w", err)
	}

	results := make(Datums, len(raw))
	// for backwards compatibility, since ogmios switched Datum values from []byte to hex string
	for k, v := range raw {
		if hexString, ok := v.(string); ok {
			results[k] = hexString
		} else {
			results[k] = hex.EncodeToString(v.([]byte))
		}
	}

	*d = results
	return nil
}

type Witness struct {
	Bootstrap  []json.RawMessage `json:"bootstrap,omitempty"  dynamodbav:"bootstrap,omitempty"`
	Datums     Datums            `json:"datums,omitempty"     dynamodbav:"datums,omitempty"`
	Redeemers  json.RawMessage   `json:"redeemers,omitempty"  dynamodbav:"redeemers,omitempty"`
	Scripts    json.RawMessage   `json:"scripts,omitempty"    dynamodbav:"scripts,omitempty"`
	Signatures map[string]string `json:"signatures,omitempty" dynamodbav:"signatures,omitempty"`
}

type ValidityInterval struct {
	InvalidBefore    uint64 `json:"invalidBefore,omitempty"    dynamodbav:"invalidBefore,omitempty"`
	InvalidHereafter uint64 `json:"invalidHereafter,omitempty" dynamodbav:"invalidHereafter,omitempty"`
}

type Value struct {
	Coins  num.Int             `json:"coins,omitempty"  dynamodbav:"coins,omitempty"`
	Assets map[AssetID]num.Int `json:"assets,omitempty" dynamodbav:"assets,omitempty"`
}

func Add(a Value, b Value) Value {
	var result Value
	result.Coins = a.Coins.Add(b.Coins)
	result.Assets = map[AssetID]num.Int{}
	for assetId, amt := range a.Assets {
		result.Assets[assetId] = amt
	}
	for assetId, amt := range b.Assets {
		result.Assets[assetId] = result.Assets[assetId].Add(amt)
	}
	return result
}
func Subtract(a Value, b Value) Value {
	var result Value
	result.Coins = a.Coins.Sub(b.Coins)
	result.Assets = map[AssetID]num.Int{}
	for assetId, amt := range a.Assets {
		result.Assets[assetId] = amt
	}
	for assetId, amt := range b.Assets {
		result.Assets[assetId] = result.Assets[assetId].Sub(amt)
	}
	return result
}
func Enough(have Value, want Value) (bool, error) {
	if have.Coins.Int64() < want.Coins.Int64() {
		return false, fmt.Errorf("not enough ADA to meet demand")
	}
	for asset, amt := range want.Assets {
		if have.Assets[asset].Int64() < amt.Int64() {
			return false, fmt.Errorf("not enough %v to meet demand", asset)
		}
	}
	return true, nil
}
func Equals(left Value, right Value) bool {
	if left.Coins.String() != right.Coins.String() {
		return false
	}
	for k, v := range left.Assets {
		lv := v.Uint64()
		if lv != 0 && right.Assets[k].Uint64() != lv {
			return false
		}
	}
	for k, v := range right.Assets {
		rv := v.Uint64()
		if rv != 0 && left.Assets[k].Uint64() != rv {
			return false
		}
	}
	return true
}
