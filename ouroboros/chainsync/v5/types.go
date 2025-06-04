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
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	"github.com/fxamacker/cbor/v2"
)

var bNil = []byte("nil")

// Use V5 materials only for JSON backwards compatibility.
type TxV5 struct {
	ID          string            `json:"id,omitempty"          dynamodbav:"id,omitempty"`
	InputSource string            `json:"inputSource,omitempty" dynamodbav:"inputSource,omitempty"`
	Body        TxBodyV5          `json:"body,omitempty"        dynamodbav:"body,omitempty"`
	Witness     chainsync.Witness `json:"witness,omitempty"     dynamodbav:"witness,omitempty"`
	Metadata    json.RawMessage   `json:"metadata,omitempty"    dynamodbav:"metadata,omitempty"`
	// Raw serialized transaction, base64.
	Raw string `json:"raw,omitempty"         dynamodbav:"raw,omitempty"`
}

// CAVEAT: v5->v6 conversion is, to some degree, best-effort-only. For example, some fields
// in v6 (e.g., "requiredExtraScripts" and "votes") either aren't represented in v5 or
// are represented such that it's very difficult, if not impossible, to determine if
// it's okay to populate the relevant fields in v6. (Example: The "scripts" field in v5
// and v6 may contain scripts that aren't considered required in v6.)
func (t TxV5) ConvertToV6() chainsync.Tx {
	withdrawals := map[string]shared.Value{}
	for txid, amt := range t.Body.Withdrawals {
		withdrawals[txid] = shared.CreateAdaValue(amt)
	}

	var tc *shared.Value
	if t.Body.TotalCollateral != nil {
		temp := shared.CreateAdaValue(*t.Body.TotalCollateral)
		tc = &temp
	}
	var cr *chainsync.TxOut
	if t.Body.CollateralReturn != nil {
		temp := t.Body.CollateralReturn.ConvertToV6()
		cr = &temp
	}

	certificates := []json.RawMessage{}
	if t.Body.Certificates != nil {
		certificates = t.Body.Certificates
	}

	// It's important to note that sigs, bootstrap or not, may be Base64. Also,
	// addressAttributes (bootstrap) may be Base64. (chainCode should be hex-only.)
	// For v6, we need to decode all Base64 sig data to hex strings.
	signatories := []chainsync.Signature{}
	for _, sig := range t.Witness.Bootstrap {
		var s chainsync.Signature
		// NOTE: error handling is ignored here, we should thread through the error
		_ = json.Unmarshal(sig, &s)

		if decodedSig, error := base64.StdEncoding.DecodeString(s.Signature); error == nil {
			s.Signature = hex.EncodeToString(decodedSig)
		}
		if s.AddressAttributes != "" {
			if decodedAtt, error := base64.StdEncoding.DecodeString(s.AddressAttributes); error == nil {
				s.AddressAttributes = hex.EncodeToString(decodedAtt)
			}
		}
		signatories = append(signatories, s)
	}
	for key, sig := range t.Witness.Signatures {
		newSig := sig
		if decodedSig, error := base64.StdEncoding.DecodeString(newSig); error == nil {
			newSig = hex.EncodeToString(decodedSig)
		}
		signatories = append(
			signatories,
			chainsync.Signature{Key: key, Signature: newSig},
		)
	}

	// Give it a sort, mostly for unit tests, so we don't intermittently fail
	sort.Slice(signatories, func(i, j int) bool {
		return signatories[i].Key < signatories[j].Key
	})

	cbor, _ := base64.StdEncoding.DecodeString(t.Raw)
	cborHex := hex.EncodeToString(cbor)
	mint := shared.Value{}
	if t.Body.Mint != nil {
		mint = t.Body.Mint.ConvertToV6()
	}
	tx := chainsync.Tx{
		ID:                       t.ID,
		Spends:                   t.InputSource,
		Inputs:                   t.Body.Inputs.ConvertToV6(),
		References:               t.Body.References.ConvertToV6(),
		Collaterals:              t.Body.Collaterals.ConvertToV6(),
		TotalCollateral:          tc,
		CollateralReturn:         cr,
		Outputs:                  t.Body.Outputs.ConvertToV6(),
		Certificates:             certificates,
		Withdrawals:              withdrawals,
		Fee:                      shared.CreateAdaValue(t.Body.Fee.Int64()),
		ValidityInterval:         t.Body.ValidityInterval.ConvertToV6(),
		Mint:                     mint,
		Network:                  t.Body.Network,
		ScriptIntegrityHash:      t.Body.ScriptIntegrityHash,
		RequiredExtraSignatories: t.Body.RequiredExtraSignatures,
		RequiredExtraScripts:     nil,
		Proposals:                t.Body.Update,
		Votes:                    nil,
		Metadata:                 t.Metadata,
		Signatories:              signatories,
		Scripts:                  t.Witness.Scripts,
		Datums:                   t.Witness.Datums,
		Redeemers:                t.Witness.Redeemers,
		CBOR:                     cborHex,
	}

	return tx
}

func TxFromV6(t chainsync.Tx) TxV5 {
	withdrawals := map[string]int64{}
	for txid, amt := range t.Withdrawals {
		for _, policyMap := range amt {
			for _, assets := range policyMap {
				withdrawals[txid] = assets.Int64()
			}
		}
	}

	var tc *int64
	if t.TotalCollateral != nil {
		temp := t.TotalCollateral.AdaLovelace().Int64()
		tc = &temp
	}
	var cr *TxOutV5
	if t.CollateralReturn != nil {
		temp := TxOutFromV6(*t.CollateralReturn)
		cr = &temp
	}

	mint := ValueFromV6(t.Mint)

	certificates := []json.RawMessage{}
	if t.Certificates != nil {
		certificates = t.Certificates
	}

	cbor, _ := hex.DecodeString(t.CBOR)
	cborB64 := base64.StdEncoding.EncodeToString(cbor)

	witness := chainsync.Witness{
		Datums:     t.Datums,
		Redeemers:  t.Redeemers,
		Scripts:    t.Scripts,
		Signatures: map[string]string{},
	}
	for _, sig := range t.Signatories {
		// Convert signatures and addressAttributes back to Base64.
		newSig := sig

		sigData, _ := hex.DecodeString(newSig.Signature)
		newSig.Signature = base64.StdEncoding.EncodeToString(sigData)
		if newSig.ChainCode != "" || newSig.AddressAttributes != "" {
			if newSig.AddressAttributes != "" {
				attrData, _ := hex.DecodeString(newSig.AddressAttributes)
				newSig.AddressAttributes = base64.StdEncoding.EncodeToString(
					attrData,
				)
			}
			s, _ := json.Marshal(newSig)
			witness.Bootstrap = append(witness.Bootstrap, s)
		} else {
			witness.Signatures[newSig.Key] = newSig.Signature
		}
	}

	network := []byte(t.Network)
	tx := TxV5{
		ID:          t.ID,
		InputSource: t.Spends,
		Body: TxBodyV5{
			Inputs:                  InputsFromV6(t.Inputs),
			References:              InputsFromV6(t.References),
			Collaterals:             InputsFromV6(t.Collaterals),
			TotalCollateral:         tc,
			CollateralReturn:        cr,
			Outputs:                 TxOutsFromV6(t.Outputs),
			Certificates:            certificates,
			Withdrawals:             withdrawals,
			Fee:                     t.Fee.AdaLovelace(),
			ValidityInterval:        ValidityIntervalFromV6(t.ValidityInterval),
			Mint:                    &mint,
			Network:                 network,
			ScriptIntegrityHash:     t.ScriptIntegrityHash,
			RequiredExtraSignatures: t.RequiredExtraSignatories,
			Update:                  t.Proposals,
		},
		Raw:      cborB64,
		Metadata: t.Metadata,
		Witness:  witness,
	}

	return tx
}

type TxBodyV5 struct {
	Certificates            []json.RawMessage  `json:"certificates,omitempty"            dynamodbav:"certificates,omitempty"`
	Collaterals             TxInsV5            `json:"collaterals,omitempty"             dynamodbav:"collaterals,omitempty"`
	Fee                     num.Int            `json:"fee,omitempty"                     dynamodbav:"fee,omitempty"`
	Inputs                  TxInsV5            `json:"inputs,omitempty"                  dynamodbav:"inputs,omitempty"`
	Mint                    *ValueV5           `json:"mint,omitempty"                    dynamodbav:"mint,omitempty"`
	Network                 json.RawMessage    `json:"network,omitempty"                 dynamodbav:"network,omitempty"`
	Outputs                 TxOutsV5           `json:"outputs,omitempty"                 dynamodbav:"outputs,omitempty"`
	RequiredExtraSignatures []string           `json:"requiredExtraSignatures,omitempty" dynamodbav:"requiredExtraSignatures,omitempty"`
	ScriptIntegrityHash     string             `json:"scriptIntegrityHash,omitempty"     dynamodbav:"scriptIntegrityHash,omitempty"`
	TimeToLive              int64              `json:"timeToLive,omitempty"              dynamodbav:"timeToLive,omitempty"`
	Update                  json.RawMessage    `json:"update,omitempty"                  dynamodbav:"update,omitempty"`
	ValidityInterval        ValidityIntervalV5 `json:"validityInterval"                  dynamodbav:"validityInterval,omitempty"`
	Withdrawals             map[string]int64   `json:"withdrawals,omitempty"             dynamodbav:"withdrawals,omitempty"`
	CollateralReturn        *TxOutV5           `json:"collateralReturn,omitempty"        dynamodbav:"collateralReturn,omitempty"`
	TotalCollateral         *int64             `json:"totalCollateral,omitempty"         dynamodbav:"totalCollateral,omitempty"`
	References              TxInsV5            `json:"references,omitempty"              dynamodbav:"references,omitempty"`
}

type TxInsV5 []TxInV5

func (t TxInsV5) ConvertToV6() chainsync.TxIns {
	txIns := chainsync.TxIns{}
	for _, txIn := range t {
		txIns = append(txIns, txIn.ConvertToV6())
	}
	return txIns
}

func InputsFromV6(t chainsync.TxIns) TxInsV5 {
	txIns := []TxInV5{}
	for _, txIn := range t {
		txIns = append(txIns, TxInV5{
			TxHash: txIn.Transaction.ID,
			Index:  txIn.Index,
		})
	}
	return txIns
}

func InputFromV6(t chainsync.TxIn) TxInV5 {
	return TxInV5{
		TxHash: t.Transaction.ID,
		Index:  t.Index,
	}
}

type TxInV5 struct {
	TxHash string `json:"txId"  dynamodbav:"txId"`
	Index  int    `json:"index" dynamodbav:"index"`
}

func (t TxInV5) String() string {
	return t.TxHash + "#" + strconv.Itoa(t.Index)
}

func (t TxInV5) ConvertToV6() chainsync.TxIn {
	id := chainsync.TxInID{ID: t.TxHash}
	return chainsync.TxIn{Transaction: id, Index: t.Index}
}

type TxOutV5 struct {
	Address   string          `json:"address,omitempty"   dynamodbav:"address,omitempty"`
	Datum     string          `json:"datum,omitempty"     dynamodbav:"datum,omitempty"`
	DatumHash string          `json:"datumHash,omitempty" dynamodbav:"datumHash,omitempty"`
	Value     ValueV5         `json:"value,omitempty"     dynamodbav:"value,omitempty"`
	Script    json.RawMessage `json:"script,omitempty"    dynamodbav:"script,omitempty"`
}

func (t TxOutV5) ConvertToV6() chainsync.TxOut {
	return chainsync.TxOut{
		Address:   t.Address,
		Datum:     t.Datum,
		DatumHash: t.DatumHash,
		Value:     t.Value.ConvertToV6(),
		Script:    t.Script,
	}
}

func TxOutFromV6(t chainsync.TxOut) TxOutV5 {
	return TxOutV5{
		Address:   t.Address,
		Datum:     t.Datum,
		DatumHash: t.DatumHash,
		Value:     ValueFromV6(t.Value),
		Script:    t.Script,
	}
}

type TxOutsV5 []TxOutV5

func (t TxOutsV5) ConvertToV6() chainsync.TxOuts {
	var txOuts []chainsync.TxOut
	for _, txOut := range t {
		txOuts = append(txOuts, txOut.ConvertToV6())
	}
	return txOuts
}

func TxOutsFromV6(t chainsync.TxOuts) TxOutsV5 {
	var txOuts []TxOutV5
	for _, txOut := range t {
		txOuts = append(txOuts, TxOutFromV6(txOut))
	}
	return txOuts
}

func (tt TxOutsV5) FindByAssetID(assetID shared.AssetID) (TxOutV5, bool) {
	for _, t := range tt {
		for gotAssetID := range t.Value.Assets {
			if gotAssetID == assetID {
				return t, true
			}
		}
	}
	return TxOutV5{}, false
}

type ValidityIntervalV5 struct {
	InvalidBefore    uint64 `json:"invalidBefore,omitempty"    dynamodbav:"invalidBefore,omitempty"`
	InvalidHereafter uint64 `json:"invalidHereafter,omitempty" dynamodbav:"invalidHereafter,omitempty"`
}

func (v ValidityIntervalV5) ConvertToV6() chainsync.ValidityInterval {
	return chainsync.ValidityInterval{
		InvalidBefore: v.InvalidBefore,
		InvalidAfter:  v.InvalidHereafter,
	}
}

func ValidityIntervalFromV6(v chainsync.ValidityInterval) ValidityIntervalV5 {
	return ValidityIntervalV5{
		InvalidBefore:    v.InvalidBefore,
		InvalidHereafter: v.InvalidAfter,
	}
}

type ValueV5 struct {
	Coins  num.Int                    `json:"coins,omitempty" dynamodbav:"coins,omitempty"`
	Assets map[shared.AssetID]num.Int `json:"assets"          dynamodbav:"assets,omitempty"`
}

func (v ValueV5) ConvertToV6() shared.Value {
	assets := shared.Value{}
	if v.Coins.Uint64() != 0 {
		assets[shared.AdaPolicy] = map[string]num.Int{
			shared.AdaAsset: v.Coins,
		}
	}
	for assetId, assetNum := range v.Assets {
		policySplit := strings.Split(string(assetId), ".")
		var (
			policyId  string
			assetName string
		)
		if len(policySplit) == 2 {
			policyId = policySplit[0]
			assetName = policySplit[1]
		} else {
			policyId = policySplit[0]
		}
		if assets[policyId] == nil {
			assets[policyId] = map[string]num.Int{}
		}
		assets[policyId][assetName] = assetNum
	}

	return assets
}

func ValueFromV6(v shared.Value) ValueV5 {
	var coins num.Int
	assets := map[shared.AssetID]num.Int{}
	for policyId, assetMap := range v {
		for assetName, assetNum := range assetMap {
			if policyId == shared.AdaPolicy && assetName == shared.AdaAsset {
				coins = assetNum
			} else {
				assetId := ""
				if assetName != "" {
					assetId = policyId + "." + assetName
				} else {
					assetId = policyId
				}
				assets[shared.AssetID(assetId)] = assetNum
			}
		}
	}
	return ValueV5{
		Coins:  coins,
		Assets: assets,
	}
}

type BlockV5 struct {
	Body       []TxV5        `json:"body,omitempty"       dynamodbav:"body,omitempty"`
	Header     BlockHeaderV5 `json:"header,omitempty"     dynamodbav:"header,omitempty"`
	HeaderHash string        `json:"headerHash,omitempty" dynamodbav:"headerHash,omitempty"`
}

type BlockHeaderV5 struct {
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

// Assume no Byron support.
func (b BlockV5) PointStruct() PointStructV5 {
	return PointStructV5{
		BlockNo: b.Header.BlockHeight,
		Hash:    b.HeaderHash,
		Slot:    b.Header.Slot,
	}
}

type PointStructV5 struct {
	BlockNo uint64 `json:"blockNo,omitempty" dynamodbav:"blockNo,omitempty"`
	Hash    string `json:"hash,omitempty"    dynamodbav:"hash,omitempty"` // BLAKE2b_256 hash
	Slot    uint64 `json:"slot,omitempty"    dynamodbav:"slot,omitempty"`
}

func (p PointStructV5) Point() PointV5 {
	return PointV5{
		pointType:   chainsync.PointTypeStruct,
		pointStruct: &p,
	}
}

func (p PointStructV5) ConvertToV6() chainsync.PointStruct {
	var bn *uint64
	if p.BlockNo != 0 {
		bn = &p.BlockNo
	}
	return chainsync.PointStruct{
		Height: bn,
		ID:     p.Hash,
		Slot:   p.Slot,
	}
}

type PointV5 struct {
	pointType   chainsync.PointType
	pointString chainsync.PointString
	pointStruct *PointStructV5
}

func (p PointV5) String() string {
	switch p.pointType {
	case chainsync.PointTypeString:
		return string(p.pointString)
	case chainsync.PointTypeStruct:
		return fmt.Sprintf(
			"slot=%v hash=%v",
			p.pointStruct.Slot,
			p.pointStruct.Hash,
		)
	default:
		return "invalid point"
	}
}

func (p PointV5) ConvertToV6() chainsync.Point {
	var p6 chainsync.Point
	if p.pointType == chainsync.PointTypeString {
		p6 = p.pointString.Point()
	} else {
		ps := chainsync.PointStruct{Slot: p.pointStruct.Slot, ID: p.pointStruct.Hash}
		p6 = ps.Point()
	}

	return p6
}

func PointFromV6(p chainsync.Point) *PointV5 {
	var p5 PointV5
	if p.PointType() == chainsync.PointTypeString {
		ps, _ := p.PointString()
		p5 = PointV5{
			pointType:   chainsync.PointTypeString,
			pointString: ps,
		}
	} else {
		ps, _ := p.PointStruct()
		bn := uint64(0)
		if ps.Height != nil {
			bn = *ps.Height
		}
		p5 = PointV5{
			pointType: chainsync.PointTypeStruct,
			pointStruct: &PointStructV5{
				BlockNo: bn,
				Hash:    ps.ID,
				Slot:    ps.Slot,
			},
		}
	}
	return &p5
}

type PointsV5 []PointV5

func (pp PointsV5) String() string {
	var ss []string
	for _, p := range pp {
		ss = append(ss, p.String())
	}
	return strings.Join(ss, ", ")
}

func (pp PointsV5) ConvertToV6() chainsync.Points {
	var points chainsync.Points
	for _, p := range pp {
		points = append(points, p.ConvertToV6())
	}
	return points
}

// pointCBOR provide simplified internal wrapper
type pointCBORV5 struct {
	String chainsync.PointString `cbor:"1,keyasint,omitempty"`
	Struct *PointStructV5        `cbor:"2,keyasint,omitempty"`
}

func (p PointV5) PointType() chainsync.PointType { return p.pointType }
func (p PointV5) PointString() (chainsync.PointString, bool) {
	return p.pointString, p.pointString != ""
}

func (p PointV5) PointStruct() (*PointStructV5, bool) { return p.pointStruct, p.pointStruct != nil }

func (p PointV5) MarshalCBOR() ([]byte, error) {
	switch p.pointType {
	case chainsync.PointTypeString, chainsync.PointTypeStruct:
		v := pointCBORV5{
			String: p.pointString,
			Struct: p.pointStruct,
		}
		return cbor.Marshal(v)
	default:
		return nil, errors.New("unable to unmarshal Point: unknown type")
	}
}

func (p PointV5) MarshalJSON() ([]byte, error) {
	switch p.pointType {
	case chainsync.PointTypeString:
		return json.Marshal(p.pointString)
	case chainsync.PointTypeStruct:
		return json.Marshal(p.pointStruct)
	default:
		return nil, errors.New("unable to unmarshal Point: unknown type")
	}
}

func (p *PointV5) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 || bytes.Equal(data, bNil) {
		return nil
	}

	var v pointCBORV5
	if err := cbor.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal Point: %w", err)
	}

	point := PointV5{
		pointType:   chainsync.PointTypeString,
		pointString: v.String,
		pointStruct: v.Struct,
	}
	if point.pointStruct != nil {
		point.pointType = chainsync.PointTypeStruct
	}

	*p = point

	return nil
}

func (p *PointV5) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf(
				"failed to unmarshal Point, %v: %w",
				string(data),
				err,
			)
		}

		*p = PointV5{
			pointType:   chainsync.PointTypeString,
			pointString: chainsync.PointString(s),
		}

	default:
		var ps PointStructV5
		if err := json.Unmarshal(data, &ps); err != nil {
			return fmt.Errorf(
				"failed to unmarshal Point, %v: %w",
				string(data),
				err,
			)
		}

		*p = PointV5{
			pointType:   chainsync.PointTypeStruct,
			pointStruct: &ps,
		}
	}

	return nil
}

// TODO: why do we have two types here?
type ResultV5 struct {
	IntersectionFound    *IntersectionFoundV5    `json:",omitempty" dynamodbav:",omitempty"`
	IntersectionNotFound *IntersectionNotFoundV5 `json:",omitempty" dynamodbav:",omitempty"`
	RollForward          *RollForwardV5          `json:",omitempty" dynamodbav:",omitempty"`
	RollBackward         *RollBackwardV5         `json:",omitempty" dynamodbav:",omitempty"`
}

type ResultFindIntersectionV5 struct {
	IntersectionFound    *IntersectionFoundV5    `json:",omitempty" dynamodbav:",omitempty"`
	IntersectionNotFound *IntersectionNotFoundV5 `json:",omitempty" dynamodbav:",omitempty"`
}

func (r ResultFindIntersectionV5) ConvertToV6() chainsync.ResultFindIntersectionPraos {
	var rfi chainsync.ResultFindIntersectionPraos
	if r.IntersectionFound != nil {
		p := r.IntersectionFound.Point.ConvertToV6()
		tip := r.IntersectionFound.Tip.ConvertToV6()
		rfi.Intersection = &p
		rfi.Tip = &tip
		rfi.Error = nil
		rfi.ID = nil
	} else if r.IntersectionNotFound != nil {
		// Emulate the v6 IntersectionNotFound error as best as possible.
		tip := r.IntersectionNotFound.Tip.ConvertToV6()
		rfi.Tip = &tip
		tipRaw, _ := json.Marshal(&tip)
		err := chainsync.ResultError{Code: 1000, Message: "Intersection not found", Data: tipRaw}
		rfi.Error = &err
	}

	return rfi
}

func ResultFindIntersectionFromV6(
	rfi chainsync.ResultFindIntersectionPraos,
) ResultFindIntersectionV5 {
	var r ResultFindIntersectionV5
	if rfi.Intersection != nil {
		p := PointFromV6(*rfi.Intersection)
		tip := PointStructV5{
			Hash: rfi.Tip.ID,
			Slot: rfi.Tip.Slot,
		}
		if rfi.Tip.Height != nil {
			tip.BlockNo = *rfi.Tip.Height
		}
		r.IntersectionFound = &IntersectionFoundV5{
			Point: p,
			Tip:   &tip,
		}
	} else if rfi.Error != nil {
		var tip PointStructV5
		_ = json.Unmarshal(rfi.Error.Data, &tip)
		r.IntersectionNotFound = &IntersectionNotFoundV5{
			Tip: &tip,
		}
	}
	return r
}

type RollBackwardV5 struct {
	Point PointV5       `json:"point,omitempty" dynamodbav:"point,omitempty"`
	Tip   PointStructV5 `json:"tip,omitempty"   dynamodbav:"tip,omitempty"`
}

type RollForwardBlockV5 struct {
	Allegra *BlockV5    `json:"allegra,omitempty" dynamodbav:"allegra,omitempty"`
	Alonzo  *BlockV5    `json:"alonzo,omitempty"  dynamodbav:"alonzo,omitempty"`
	Babbage *BlockV5    `json:"babbage,omitempty" dynamodbav:"babbage,omitempty"`
	Byron   *ByronBlock `json:"byron,omitempty"   dynamodbav:"byron,omitempty"`
	Mary    *BlockV5    `json:"mary,omitempty"    dynamodbav:"mary,omitempty"`
	Shelley *BlockV5    `json:"shelley,omitempty" dynamodbav:"shelley,omitempty"`
}

func (b RollForwardBlockV5) Era() string {
	if b.Shelley != nil {
		return "shelley"
	} else if b.Allegra != nil {
		return "allegra"
	} else if b.Mary != nil {
		return "mary"
	} else if b.Alonzo != nil {
		return "alonzo"
	} else if b.Babbage != nil {
		return "babbage"
	} else {
		return "unknown"
	}
}

func (b RollForwardBlockV5) GetNonByronBlock() *BlockV5 {
	if b.Shelley != nil {
		return b.Shelley
	} else if b.Allegra != nil {
		return b.Allegra
	} else if b.Mary != nil {
		return b.Mary
	} else if b.Alonzo != nil {
		return b.Alonzo
	} else if b.Babbage != nil {
		return b.Babbage
	} else {
		return nil
	}
}

func (b RollForwardBlockV5) ConvertToV6() (chainsync.Block, error) {
	nbb := b.GetNonByronBlock()
	if nbb == nil {
		return chainsync.Block{}, errors.New("byron blocks not supported")
	}
	var txArray []chainsync.Tx
	for _, t := range nbb.Body {
		txArray = append(txArray, t.ConvertToV6())
	}

	// The v5 spec indicates that both nonce entries are optional. We'll create a v6
	// entry (which also indicates both are optional) if either is present.
	nonceOutput := nbb.Header.Nonce["output"]
	nonceProof := nbb.Header.Nonce["output"]
	var nonce *chainsync.Nonce
	if nonceOutput != "" || nonceProof != "" {
		nonce = &chainsync.Nonce{Output: nonceOutput, Proof: nonceProof}
	}
	majorVer := uint32(nbb.Header.ProtocolVersion["major"])
	protocolVersion := chainsync.ProtocolVersion{
		Major: majorVer,
		Minor: uint32(nbb.Header.ProtocolVersion["minor"]),
		Patch: uint32(nbb.Header.ProtocolVersion["patch"]),
	}
	protocol := chainsync.Protocol{Version: protocolVersion}

	var opCert chainsync.OpCert
	if nbb.Header.OpCert != nil {
		var vk []byte
		if nbb.Header.OpCert["hotVk"] != nil {
			vk, _ = base64.StdEncoding.DecodeString(
				nbb.Header.OpCert["hotVk"].(string),
			)
		}
		count := nbb.Header.OpCert["count"]
		kesPd := nbb.Header.OpCert["kesPeriod"]

		// Yes, the uint64 casts are ugly. JSON covers floats but not ints. Unmarshalling
		// into interface{} creates float64. If we treat interface{} as uint64, the code
		// compiles but crashes at runtime. So, we cast float64 to uint64.
		opCert = chainsync.OpCert{
			Count: uint64(count.(float64)),
			Kes: chainsync.Kes{
				Period:          uint64(kesPd.(float64)),
				VerificationKey: string(vk),
			},
		}
	}

	// TODO: this might be wrong
	var leaderValue *chainsync.LeaderValue
	if nbb.Header.LeaderValue["output"] != nil &&
		nbb.Header.LeaderValue["proof"] != nil {
		decodedOutput, _ := base64.StdEncoding.DecodeString(
			string(nbb.Header.LeaderValue["output"]),
		)
		decodedProof, _ := base64.StdEncoding.DecodeString(
			string(nbb.Header.LeaderValue["proof"]),
		)
		leaderValue = &chainsync.LeaderValue{
			Output: string(decodedOutput),
			Proof:  string(decodedProof),
		}
	}

	issuerVrf, _ := base64.StdEncoding.DecodeString(nbb.Header.IssuerVrf)
	issuer := chainsync.BlockIssuer{
		VerificationKey:        nbb.Header.IssuerVK,
		VrfVerificationKey:     string(issuerVrf),
		OperationalCertificate: opCert,
		LeaderValue:            leaderValue,
	}

	return chainsync.Block{
		Type:         "praos",
		Era:          b.Era(),
		ID:           nbb.HeaderHash,
		Ancestor:     nbb.Header.PrevHash,
		Nonce:        nonce,
		Height:       nbb.Header.BlockHeight,
		Size:         chainsync.BlockSize{Bytes: int64(nbb.Header.BlockSize)},
		Slot:         nbb.Header.Slot,
		Transactions: txArray,
		Protocol:     protocol,
		Issuer:       issuer,
	}, nil
}

func BlockFromV6(b chainsync.Block) (RollForwardBlockV5, error) {
	if b.Era == "byron" {
		return RollForwardBlockV5{}, errors.New("byron blocks not supported")
	}

	var txArray []TxV5
	for _, t := range b.Transactions {
		txArray = append(txArray, TxFromV6(t))
	}

	var nonce map[string]string
	if b.Nonce != nil {
		nonce = map[string]string{
			"output": b.Nonce.Output,
			"proof":  b.Nonce.Proof,
		}
	}
	protocolVersion := b.Protocol.Version

	vkey, _ := hex.DecodeString(
		b.Issuer.OperationalCertificate.Kes.VerificationKey,
	)
	hotVk := base64.StdEncoding.EncodeToString(vkey)
	opCert := map[string]interface{}{
		"hotVk":     hotVk,
		"count":     b.Issuer.OperationalCertificate.Count,
		"kesPeriod": b.Issuer.OperationalCertificate.Kes.Period,
	}

	// TODO: this might be wrong
	output := ""
	proof := ""
	if b.Issuer.LeaderValue != nil {
		output = base64.StdEncoding.EncodeToString(
			[]byte(b.Issuer.LeaderValue.Output),
		)
		proof = base64.StdEncoding.EncodeToString(
			[]byte(b.Issuer.LeaderValue.Proof),
		)
	}
	leaderValue := map[string][]byte{
		"output": []byte(output),
		"proof":  []byte(proof),
	}

	bv5 := BlockV5{
		Body: txArray,
		Header: BlockHeaderV5{
			Nonce: nonce,
			ProtocolVersion: map[string]int{
				"major": int(protocolVersion.Major),
				"minor": int(protocolVersion.Minor),
				"patch": int(protocolVersion.Patch),
			},
			OpCert:      opCert,
			LeaderValue: leaderValue,
			IssuerVK:    b.Issuer.VerificationKey,
			IssuerVrf:   b.Issuer.VrfVerificationKey,
			PrevHash:    b.Ancestor,
			Slot:        b.Slot,
			BlockHeight: b.Height,
			BlockSize:   uint64(b.Size.Bytes),
			BlockHash:   b.ID,
		},
		HeaderHash: b.ID,
	}

	switch b.Era {
	case "shelley":
		return RollForwardBlockV5{
			Shelley: &bv5,
		}, nil
	case "allegra":
		return RollForwardBlockV5{
			Allegra: &bv5,
		}, nil
	case "mary":
		return RollForwardBlockV5{
			Mary: &bv5,
		}, nil
	case "alonzo":
		return RollForwardBlockV5{
			Alonzo: &bv5,
		}, nil
	case "babbage":
		return RollForwardBlockV5{
			Babbage: &bv5,
		}, nil
	default:
		return RollForwardBlockV5{}, fmt.Errorf("unknown era: %v", b.Era)
	}
}

type RollForwardV5 struct {
	Block RollForwardBlockV5 `json:"block,omitempty" dynamodbav:"block,omitempty"`
	Tip   PointStructV5      `json:"tip,omitempty"   dynamodbav:"tip,omitempty"`
}

type ResultNextBlockV5 struct {
	RollForward  *RollForwardV5  `json:",omitempty" dynamodbav:",omitempty"`
	RollBackward *RollBackwardV5 `json:",omitempty" dynamodbav:",omitempty"`
}

func (r ResultNextBlockV5) ConvertToV6() chainsync.ResultNextBlockPraos {
	var rnb chainsync.ResultNextBlockPraos
	if r.RollForward != nil {
		tip := r.RollForward.Tip.ConvertToV6()
		block, err := r.RollForward.Block.ConvertToV6()
		if err != nil {
			// NOTE: we currently don't support byron blocks, please reach out if you need this!
			return rnb
		}
		rnb.Direction = chainsync.RollForwardString
		rnb.Tip = &tip
		rnb.Block = &block
	} else if r.RollBackward != nil {
		tip := r.RollBackward.Tip.ConvertToV6()
		point := r.RollBackward.Point.ConvertToV6()
		rnb.Direction = chainsync.RollBackwardString
		rnb.Tip = &tip
		rnb.Point = &point
	}

	return rnb
}

func ResultNextBlockFromV6(
	rnb chainsync.ResultNextBlockPraos,
) ResultNextBlockV5 {
	var r ResultNextBlockV5
	switch rnb.Direction {
	case chainsync.RollForwardString:
		tip := PointStructV5{
			Hash: rnb.Tip.ID,
			Slot: rnb.Tip.Slot,
		}
		if rnb.Tip.Height != nil {
			tip.BlockNo = *rnb.Tip.Height
		}
		block, err := BlockFromV6(*rnb.Block)
		if err != nil {
			// NOTE: we don't currently support byron
			return r
		}
		r.RollForward = &RollForwardV5{
			Block: block,
			Tip:   tip,
		}
	case chainsync.RollBackwardString:
		tip := PointStructV5{
			Hash: rnb.Tip.ID,
			Slot: rnb.Tip.Slot,
		}
		if rnb.Tip.Height != nil {
			tip.BlockNo = *rnb.Tip.Height
		}
		r.RollBackward = &RollBackwardV5{
			Point: *PointFromV6(*rnb.Point),
			Tip:   tip,
		}
	}

	return r
}

type IntersectionFoundV5 struct {
	Point *PointV5
	Tip   *PointStructV5
}

type IntersectionNotFoundV5 struct {
	Tip *PointStructV5
}

type ResponseV5 struct {
	Type        string          `json:"type,omitempty"        dynamodbav:"type,omitempty"`
	Version     string          `json:"version,omitempty"     dynamodbav:"version,omitempty"`
	ServiceName string          `json:"servicename,omitempty" dynamodbav:"servicename,omitempty"`
	MethodName  string          `json:"methodname,omitempty"  dynamodbav:"methodname,omitempty"`
	Result      *ResultV5       `json:"result,omitempty"      dynamodbav:"result,omitempty"`
	Reflection  json.RawMessage `json:"reflection,omitempty"  dynamodbav:"reflection,omitempty"`
}

func (r ResponseV5) ConvertToV6() chainsync.ResponsePraos {
	var c chainsync.ResponsePraos

	// All we really care about is the result, not the metadata.
	if r.Result.IntersectionFound != nil {
		c.Method = chainsync.FindIntersectionMethod

		p := r.Result.IntersectionFound.Point.ConvertToV6()
		t := r.Result.IntersectionFound.Tip.ConvertToV6()

		var findIntersection chainsync.ResultFindIntersectionPraos
		findIntersection.Intersection = &p
		findIntersection.Tip = &t
		c.Result = &findIntersection
	} else if r.Result.IntersectionNotFound != nil {
		c.Method = chainsync.FindIntersectionMethod
		t := r.Result.IntersectionNotFound.Tip.ConvertToV6()
		tRaw, _ := json.Marshal(&t)
		var e chainsync.ResultError
		e.Data = tRaw
		e.Code = 1000
		e.Message = "Intersection not found - Conversion from a v5 Ogmigo call"
		c.Error = &e
	} else if r.Result.RollForward != nil {
		c.Method = chainsync.NextBlockMethod

		block, err := r.Result.RollForward.Block.ConvertToV6()
		if err != nil {
			// NOTE: we currently don't support byron, reach out to us if you need this supported!
			return c
		}

		t := r.Result.RollForward.Tip.ConvertToV6()

		var nextBlock chainsync.ResultNextBlockPraos
		nextBlock.Direction = chainsync.RollForwardString
		nextBlock.Tip = &t
		nextBlock.Block = &block
		c.Result = &nextBlock
	} else if r.Result.RollBackward != nil {
		c.Method = chainsync.NextBlockMethod
		var t chainsync.PointStruct
		t.Slot = r.Result.RollBackward.Tip.Slot
		t.ID = r.Result.RollBackward.Tip.Hash
		if r.Result.RollBackward.Tip.BlockNo != 0 {
			t.Height = &r.Result.RollBackward.Tip.BlockNo
		}

		p := r.Result.RollBackward.Point.ConvertToV6()
		var nextBlock chainsync.ResultNextBlockPraos
		nextBlock.Direction = chainsync.RollBackwardString
		nextBlock.Tip = &t
		nextBlock.Point = &p
		c.Result = &nextBlock
	}
	c.ID = r.Reflection
	c.JsonRpc = "2.0"
	return c
}

// I don't really understand the nest of types here...
func ResponseFromV6(r chainsync.ResponsePraos) ResponseV5 {
	var result *ResultV5
	switch r.Method {
	case chainsync.FindIntersectionMethod:
		rfi := ResultFindIntersectionFromV6(r.MustFindIntersectResult())
		if rfi.IntersectionFound != nil {
			result = &ResultV5{
				IntersectionFound: rfi.IntersectionFound,
			}
		} else {
			result = &ResultV5{
				IntersectionNotFound: rfi.IntersectionNotFound,
			}
		}
	case chainsync.NextBlockMethod:
		rnb := ResultNextBlockFromV6(r.MustNextBlockResult())
		if rnb.RollForward != nil {
			result = &ResultV5{
				RollForward: rnb.RollForward,
			}
		} else {
			result = &ResultV5{
				RollBackward: rnb.RollBackward,
			}
		}
	}

	return ResponseV5{
		Type:        "response",
		Version:     "1.0",
		ServiceName: "cardano",
		MethodName:  "cardano",
		Result:      result,
		Reflection:  r.ID,
	}
}

type OgmiosAuxiliaryDataV5Body struct {
	Blob OgmiosMetadataV5 `json:"blob"`
}

type OgmiosMetadataV5 map[int]chainsync.OgmiosMetadatum

type OgmiosAuxiliaryDataV5 struct {
	Hash string                     `json:"hash"`
	Body *OgmiosAuxiliaryDataV5Body `json:"body"`
}

func GetMetadataDatumsV5(
	txMetadata json.RawMessage,
	metadataDatumKey int,
) ([][]byte, error) {
	datums, err := GetMetadataDatumMapV5(txMetadata, metadataDatumKey)
	if err != nil {
		return nil, err
	}
	return chainsync.GetMetadataDatums(datums)
}

func GetMetadataDatumMapV5(
	txMetadata json.RawMessage,
	metadataDatumKey int,
) (map[string][]byte, error) {
	// Ogmios will sometimes set the Metadata field to "null" when there's not
	// any actual metadata. This can lead to unintended errors. If we encounter
	// this case, just return an empty map.
	if bytes.Equal(txMetadata, json.RawMessage("null")) {
		var dummyMap map[string][]byte
		return dummyMap, nil
	}

	var auxData OgmiosAuxiliaryDataV5
	err := json.Unmarshal(txMetadata, &auxData)
	if err != nil {
		return nil, err
	}
	dats, ok := auxData.Body.Blob[metadataDatumKey]
	if !ok {
		return nil, nil
	}
	return chainsync.ReconstructDatums(dats)
}

func (t OgmiosAuxiliaryDataV5) ConvertToV6() chainsync.OgmiosAuxiliaryDataV6 {
	labels := make(chainsync.OgmiosAuxiliaryDataLabelsV6)
	for k, v := range t.Body.Blob {
		metadatum := chainsync.OgmiosMetadatumRecordV6{
			Json: &v,
		}
		labels[k] = metadatum
	}

	return chainsync.OgmiosAuxiliaryDataV6{
		Hash:   t.Hash,
		Labels: &labels,
	}
}

// NOTE: This works only for JSON metadata. Entries with CBOR metadata are ignored.
func OgmiosAuxiliaryDataFromV6(
	t chainsync.OgmiosAuxiliaryDataV6,
) (OgmiosAuxiliaryDataV5, error) {
	if t.Labels == nil {
		return OgmiosAuxiliaryDataV5{}, nil
	}

	labels := *(t.Labels)
	blob := make(OgmiosMetadataV5)
	for k, v := range labels {
		if v.Json != nil {
			blob[k] = *v.Json
		}
	}

	return OgmiosAuxiliaryDataV5{
		Hash: t.Hash,
		Body: &OgmiosAuxiliaryDataV5Body{
			Blob: blob,
		},
	}, nil
}
