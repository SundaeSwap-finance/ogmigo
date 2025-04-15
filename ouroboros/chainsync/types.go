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
	"math/big"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/fxamacker/cbor/v2"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/shared"
)

var (
	bNil = []byte("nil")
)

// All blocks except Byron-era blocks.
type Block struct {
	Type         string      `json:"type,omitempty"`
	Era          string      `json:"era,omitempty"`
	ID           string      `json:"id,omitempty"`
	Ancestor     string      `json:"ancestor,omitempty"`
	Nonce        *Nonce      `json:"nonce,omitempty"`
	Height       uint64      `json:"height,omitempty"`
	Size         BlockSize   `json:"size,omitempty"`
	Slot         uint64      `json:"slot,omitempty"`
	Transactions []Tx        `json:"transactions,omitempty"`
	Protocol     Protocol    `json:"protocol,omitempty"`
	Issuer       BlockIssuer `json:"issuer,omitempty"`
}

type Nonce struct {
	Output string `json:"output,omitempty" dynamodbav:"slot,omitempty"`
	Proof  string `json:"proof,omitempty"  dynamodbav:"slot,omitempty"`
}

type BlockSize struct {
	Bytes int64 `json:"bytes,omitempty"  dynamodbav:"bytes,omitempty"`
}

type Protocol struct {
	Version ProtocolVersion `json:"version,omitempty" dynamodbav:"version,omitempty"`
}

type BlockIssuer struct {
	VerificationKey        string       `json:"verificationKey,omitempty"`
	VrfVerificationKey     string       `json:"vrfVerificationKey,omitempty"`
	OperationalCertificate OpCert       `json:"operationalCertificate,omitempty"`
	LeaderValue            *LeaderValue `json:"leaderValue,omitempty"`
}

type OpCert struct {
	Count uint64 `json:"count,omitempty"`
	Kes   Kes    `json:"kes,omitempty"`
}

type Kes struct {
	Period          uint64 `json:"period,omitempty"`
	VerificationKey string `json:"verificationKey,omitempty"`
}

type LeaderValue struct {
	Output string `json:"output,omitempty"`
	Proof  string `json:"proof,omitempty"`
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
	Height *uint64 `json:"height,omitempty" dynamodbav:"height,omitempty"` // Not part of RollBackward.
	ID     string  `json:"id,omitempty"      dynamodbav:"id,omitempty"`    // BLAKE2b_256 hash
	Slot   uint64  `json:"slot,omitempty"    dynamodbav:"slot,omitempty"`
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
		if p.pointStruct.Height == nil {
			return fmt.Sprintf("slot=%v id=%v", p.pointStruct.Slot, p.pointStruct.ID)
		}
		return fmt.Sprintf("slot=%v id=%v block=%v", p.pointStruct.Slot, p.pointStruct.ID, *p.pointStruct.Height)
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
	Direction string            `json:"direction,omitempty" dynamodbav:"direction,omitempty"`
	Tip       PointStruct       `json:"tip,omitempty"   dynamodbav:"tip,omitempty"`
	Point     RollBackwardPoint `json:"point,omitempty" dynamodbav:"point,omitempty"`
}

type RollBackwardPoint struct {
	Slot uint64 `json:"slot,omitempty"    dynamodbav:"slot,omitempty"`
	ID   string `json:"id,omitempty"      dynamodbav:"id,omitempty"` // BLAKE2b_256 hash
}

// Assume non-Byron blocks.
type RollForward struct {
	Direction string      `json:"direction,omitempty" dynamodbav:"direction,omitempty"`
	Tip       PointStruct `json:"tip,omitempty"   dynamodbav:"tip,omitempty"`
	Block     Block       `json:"block,omitempty" dynamodbav:"block,omitempty"`
}

func (b Block) PointStruct() PointStruct {
	return PointStruct{
		Height: &b.Height,
		ID:     b.ID,
		Slot:   b.Slot,
	}
}

// Covers everything except Byron-era blocks.
type ResultFindIntersectionPraos struct {
	Intersection *Point          `json:"intersection,omitempty" dynamodbav:"intersection,omitempty"`
	Tip          *PointStruct    `json:"tip,omitempty"          dynamodbav:"tip,omitempty"`
	Error        *ResultError    `json:"error,omitempty"        dynamodbav:"error,omitempty"`
	ID           json.RawMessage `json:"id,omitempty"           dynamodbav:"id,omitempty"`
}

type ResultError struct {
	Code    uint32          `json:"code,omitempty"    dynamodbav:"code,omitempty"`
	Message string          `json:"message,omitempty" dynamodbav:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"    dynamodbav:"data,omitempty"` // Forward
	ID      json.RawMessage `json:"id,omitempty"      dynamodbav:"id,omitempty"`
}

// Covers all blocks except Byron-era blocks.
type ResultNextBlockPraos struct {
	Direction string       `json:"direction,omitempty" dynamodbav:"direction,omitempty"`
	Tip       *PointStruct `json:"tip,omitempty"       dynamodbav:"tip,omitempty"`
	Block     *Block       `json:"block,omitempty"     dynamodbav:"block,omitempty"` // Forward
	Point     *Point       `json:"point,omitempty"     dynamodbav:"point,omitempty"` // Backward
}

type ResponsePraos struct {
	JsonRpc string          `json:"jsonrpc,omitempty" dynamodbav:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"  dynamodbav:"method,omitempty"`
	Result  interface{}     `json:"result,omitempty"  dynamodbav:"result,omitempty"`
	Error   *ResultError    `json:"error,omitempty"   dynamodbav:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"      dynamodbav:"id,omitempty"`
}

const FindIntersectionMethod = "findIntersection"
const NextBlockMethod = "nextBlock"
const FindIntersectMethod = "FindIntersect"
const RequestNextMethod = "RequestNext"

const RollForwardString = "forward"
const RollBackwardString = "backward"

func (r *ResponsePraos) UnmarshalJSON(b []byte) error {
	var m struct {
		JsonRpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		ID      json.RawMessage `json:"ID"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	r.JsonRpc = m.JsonRpc
	r.ID = m.ID

	if m.Error != nil {
		var resultError ResultError
		if err := json.Unmarshal(m.Error, &resultError); err != nil {
			return err
		}
		r.Error = &resultError
	} else {
		switch m.Method {
		case FindIntersectionMethod, FindIntersectMethod:
			r.Method = FindIntersectionMethod
			var findIntersection ResultFindIntersectionPraos
			if err := json.Unmarshal(m.Result, &findIntersection); err != nil {
				return err
			}
			r.Result = findIntersection

		case NextBlockMethod, RequestNextMethod:
			r.Method = NextBlockMethod
			var nextBlock ResultNextBlockPraos
			if err := json.Unmarshal(m.Result, &nextBlock); err != nil {
				return err
			}
			r.Result = nextBlock

		default:
			return fmt.Errorf("unknown method: '%v'", r.Method)
		}
	}

	return nil
}

func (r ResponsePraos) MustFindIntersectResult() ResultFindIntersectionPraos {
	if r.Method != FindIntersectionMethod {
		panic(fmt.Errorf("must only use *Must* methods after switching on the findIntersection method; called on %v", r.Method))
	}
	t, ok := r.Result.(ResultFindIntersectionPraos)
	if ok {
		return t
	}
	u, ok := r.Result.(*ResultFindIntersectionPraos)
	if ok && u != nil {
		return *u
	}
	panic(fmt.Errorf("must method used on incompatible type"))
}

func (r ResponsePraos) MustNextBlockResult() ResultNextBlockPraos {
	if r.Method != NextBlockMethod {
		panic(fmt.Errorf("must only use *Must* methods after switching on the nextBlock method; called on %v", r.Method))
	}
	t, ok := r.Result.(ResultNextBlockPraos)
	if ok {
		return t
	}
	u, ok := r.Result.(*ResultNextBlockPraos)
	if ok && u != nil {
		return *u
	}
	panic(fmt.Errorf("must method used on incompatible type"))
}

type Signature struct {
	Key               string `json:"key" dynamodbav:"key"`
	Signature         string `json:"signature" dynamodbav:"signature"`
	ChainCode         string `json:"chainCode,omitempty" dynamodbav:"chainCode,omitempty"`
	AddressAttributes string `json:"addressAttributes,omitempty" dynamodbav:"addressAttributes,omitempty"`
}

type Tx struct {
	ID                       string                  `json:"id,omitempty"                       dynamodbav:"id,omitempty"`
	Spends                   string                  `json:"spends,omitempty"                   dynamodbav:"spends,omitempty"`
	Inputs                   []TxIn                  `json:"inputs,omitempty"                   dynamodbav:"inputs,omitempty"`
	References               []TxIn                  `json:"references,omitempty"               dynamodbav:"references,omitempty"`
	Collaterals              []TxIn                  `json:"collaterals,omitempty"              dynamodbav:"collaterals,omitempty"`
	TotalCollateral          *shared.Value           `json:"totalCollateral,omitempty"          dynamodbav:"totalCollateral,omitempty"`
	CollateralReturn         *TxOut                  `json:"collateralReturn,omitempty"         dynamodbav:"collateralReturn,omitempty"`
	Outputs                  TxOuts                  `json:"outputs,omitempty"                  dynamodbav:"outputs,omitempty"`
	Certificates             []json.RawMessage       `json:"certificates,omitempty"             dynamodbav:"certificates,omitempty"`
	Withdrawals              map[string]shared.Value `json:"withdrawals,omitempty"              dynamodbav:"withdrawals,omitempty"`
	Fee                      shared.Value            `json:"fee,omitempty"                      dynamodbav:"fee,omitempty"`
	ValidityInterval         ValidityInterval        `json:"validityInterval"                   dynamodbav:"validityInterval,omitempty"`
	Mint                     shared.Value            `json:"mint,omitempty"                     dynamodbav:"mint,omitempty"`
	Network                  json.RawMessage         `json:"network,omitempty"                  dynamodbav:"network,omitempty"`
	ScriptIntegrityHash      string                  `json:"scriptIntegrityHash,omitempty"      dynamodbav:"scriptIntegrityHash,omitempty"`
	RequiredExtraSignatories []string                `json:"requiredExtraSignatories,omitempty" dynamodbav:"requiredExtraSignatories,omitempty"`
	RequiredExtraScripts     []string                `json:"requiredExtraScripts,omitempty"     dynamodbav:"requiredExtraScripts,omitempty"`
	Proposals                json.RawMessage         `json:"proposals,omitempty"                dynamodbav:"proposals,omitempty"`
	Votes                    json.RawMessage         `json:"votes,omitempty"                    dynamodbav:"votes,omitempty"`
	Metadata                 json.RawMessage         `json:"metadata,omitempty"                 dynamodbav:"metadata,omitempty"`
	Signatories              []Signature             `json:"signatories,omitempty"              dynamodbav:"signatories,omitempty"`
	Scripts                  json.RawMessage         `json:"scripts,omitempty"                  dynamodbav:"scripts,omitempty"`
	Datums                   Datums                  `json:"datums"                             dynamodbav:"datums,omitempty"`
	Redeemers                json.RawMessage         `json:"redeemers,omitempty"                dynamodbav:"redeemers,omitempty"`
	CBOR                     string                  `json:"cbor,omitempty"                     dynamodbav:"cbor,omitempty"`
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
	Transaction TxInID `json:"transaction"  dynamodbav:"transaction"`
	Index       int    `json:"index" dynamodbav:"index"`
}

type TxIns []TxIn

type TxInID struct {
	ID string `json:"id"  dynamodbav:"id"`
}

func (t TxIn) String() string {
	return t.Transaction.ID + "#" + strconv.Itoa(t.Index)
}

func (t TxIn) TxID() TxID {
	return NewTxID(t.Transaction.ID, t.Index)
}

type TxOut struct {
	Address   string          `json:"address,omitempty"   dynamodbav:"address,omitempty"`
	Datum     string          `json:"datum,omitempty"     dynamodbav:"datum,omitempty"`
	DatumHash string          `json:"datumHash,omitempty" dynamodbav:"datumHash,omitempty"`
	Value     shared.Value    `json:"value,omitempty"     dynamodbav:"value,omitempty"`
	Script    json.RawMessage `json:"script,omitempty"    dynamodbav:"script,omitempty"`
}

type TxOuts []TxOut

type Datums map[string]string

type TxInQuery struct {
	Transaction shared.UtxoTxID `json:"transaction"  dynamodbav:"transaction"`
	Index       uint32          `json:"index" dynamodbav:"index"`
}

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
	Datums     Datums            `json:"datums"     dynamodbav:"datums,omitempty"`
	Redeemers  json.RawMessage   `json:"redeemers,omitempty"  dynamodbav:"redeemers,omitempty"`
	Scripts    json.RawMessage   `json:"scripts,omitempty"    dynamodbav:"scripts,omitempty"`
	Signatures map[string]string `json:"signatures,omitempty" dynamodbav:"signatures,omitempty"`
}

type ValidityInterval struct {
	InvalidBefore uint64 `json:"invalidBefore,omitempty" dynamodbav:"invalidBefore,omitempty"`
	InvalidAfter  uint64 `json:"invalidAfter,omitempty"  dynamodbav:"invalidAfter,omitempty"`
}

type OgmiosAuxiliaryData struct {
	Hash   string
	Labels *OgmiosAuxiliaryDataLabels
}

type OgmiosAuxiliaryDataLabels map[int]OgmiosMetadatumRecord

func (o *OgmiosAuxiliaryData) UnmarshalJSON(data []byte) error {
	var s struct {
		Hash   string
		Labels *OgmiosAuxiliaryDataLabels
	}
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	if s.Hash == "" {
		return fmt.Errorf("OgmiosAuxiliaryData: UnmarshalJSON: Hash is empty")
	}
	o.Hash = s.Hash
	o.Labels = s.Labels
	return nil
}

type OgmiosMetadatumRecord struct {
	Cbor *string          `json:"cbor"`
	Json *OgmiosMetadatum `json:"json"`
}

type OgmiosMetadatumKind int

type OgmiosMetadatum struct {
	Tag         OgmiosMetadatumKind
	IntField    *big.Int              `json:"int"`
	StringField string                `json:"string"`
	BytesField  []byte                `json:"bytes"`
	ListField   []*OgmiosMetadatum    `json:"list"`
	MapField    []*OgmiosMetadatumMap `json:"map"`
}

const (
	OgmiosMetadatumTagUnknown OgmiosMetadatumKind = iota
	OgmiosMetadatumTagInt
	OgmiosMetadatumTagString
	OgmiosMetadatumTagBytes
	OgmiosMetadatumTagList
	OgmiosMetadatumTagMap
)

func (o *OgmiosMetadatum) UnmarshalJSON(data []byte) error {
	type intField struct {
		X *big.Int `json:"int"`
	}
	var i intField
	if err := json.Unmarshal(data, &i); err == nil && i.X != nil {
		o.Tag = OgmiosMetadatumTagInt
		o.IntField = i.X
		return nil
	}
	type stringField struct {
		X *string `json:"string"`
	}
	var s stringField
	if err := json.Unmarshal(data, &s); err == nil && s.X != nil {
		o.Tag = OgmiosMetadatumTagString
		o.StringField = *s.X
		return nil
	}
	type bytesField struct {
		X *string `json:"bytes"`
	}
	var b bytesField
	if err := json.Unmarshal(data, &b); err == nil && b.X != nil {
		dec, err := hex.DecodeString(*b.X)
		if err != nil {
			return err
		}
		o.Tag = OgmiosMetadatumTagBytes
		o.BytesField = dec
		return nil
	}
	type listField struct {
		X *[]*OgmiosMetadatum `json:"list"`
	}
	var l listField
	if err := json.Unmarshal(data, &l); err == nil && l.X != nil {
		o.Tag = OgmiosMetadatumTagList
		o.ListField = *l.X
		return nil
	}
	type mapField struct {
		X *[]*OgmiosMetadatumMap `json:"map"`
	}
	var m mapField
	if err := json.Unmarshal(data, &m); err == nil && m.X != nil {
		o.Tag = OgmiosMetadatumTagMap
		o.MapField = *m.X
		return nil
	}
	return fmt.Errorf("Can't unmarshal %s as OgmiosMetadatum", data)
}

type OgmiosMetadatumMap struct {
	Key   *OgmiosMetadatum `json:"k"`
	Value *OgmiosMetadatum `json:"v"`
}

func GetMetadataDatums(datums map[string][]byte) ([][]byte, error) {
	var datumBytes [][]byte
	for _, datum := range datums {
		datumBytes = append(datumBytes, datum)
	}
	return datumBytes, nil
}

func GetMetadataDatumMap(txMetadata json.RawMessage, metadataDatumKey int) (map[string][]byte, error) {
	// Ogmios will sometimes set the Metadata field to "null" when there's not
	// any actual metadata. This can lead to unintended errors. If we encounter
	// this case, just return an empty map.
	if bytes.Equal(txMetadata, json.RawMessage("null")) {
		var dummyMap map[string][]byte
		return dummyMap, nil
	}

	var auxData OgmiosAuxiliaryData
	err := json.Unmarshal(txMetadata, &auxData)
	if err != nil {
		return nil, err
	}
	labels := *(auxData.Labels)
	dats, ok := labels[metadataDatumKey]
	if !ok {
		return nil, nil
	}
	if dats.Json == nil {
		return nil, fmt.Errorf("transaction metadata at key '%d' is missing a json representation: '%v' (is ogmios running with --metadata-detailed-schema?)", metadataDatumKey, string(txMetadata))
	}
	return ReconstructDatums(*(dats.Json))
}

func ReconstructDatums(metadatum OgmiosMetadatum) (map[string][]byte, error) {
	newDatums := make(map[string][]byte, 0)
	switch metadatum.Tag {
	case OgmiosMetadatumTagMap:
		for _, mapItem := range metadatum.MapField {
			k := mapItem.Key
			switch k.Tag {
			case OgmiosMetadatumTagBytes:
				reconstructed := make([]byte, 0)
				v := mapItem.Value
				switch v.Tag {
				case OgmiosMetadatumTagList:
					for _, chunk := range v.ListField {
						reconstructed = append(reconstructed, chunk.BytesField...)
					}
					newDatums[hex.EncodeToString(k.BytesField)] = reconstructed
				default: // Misformed, ignore
					continue
				}
			default: // Misformed, ignore
				continue
			}
		}
	default: // Misformed, ignore
		fmt.Print("Misformed")
		return nil, nil
	}
	return newDatums, nil
}
