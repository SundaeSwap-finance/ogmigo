package shared

import (
	"errors"
	"fmt"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
)

type Value map[string]map[string]num.Int

var ErrInsufficientFunds = errors.New("insufficient funds")

func Add(a Value, b Value) Value {
	result := Value{}
	for policyId, assets := range a {
		for assetName, amt := range assets {
			if _, ok := result[policyId]; !ok {
				result[policyId] = map[string]num.Int{}
			}
			result[policyId][assetName] = amt
		}
	}
	for policyId, assets := range b {
		for assetName, amt := range assets {
			if _, ok := result[policyId]; !ok {
				result[policyId] = map[string]num.Int{}
			}
			result[policyId][assetName] = result[policyId][assetName].Add(amt)
		}
	}

	return result
}

func Subtract(a Value, b Value) Value {
	result := Value{}
	for policyId, assets := range a {
		for assetName, amt := range assets {
			if _, ok := result[policyId]; !ok {
				result[policyId] = map[string]num.Int{}
			}
			result[policyId][assetName] = amt
		}
	}
	for policyId, assets := range b {
		for assetName, amt := range assets {
			if _, ok := result[policyId]; !ok {
				result[policyId] = map[string]num.Int{}
			}
			result[policyId][assetName] = result[policyId][assetName].Sub(amt)
		}
	}

	return result
}

func Enough(have Value, want Value) (bool, error) {
	for policyId, assets := range want {
		for assetName, amt := range assets {
			haveAssets, ok := have[policyId]
			haveAmt := num.Uint64(0)
			if ok {
				haveAmt = haveAssets[assetName]
			}
			if haveAmt.LessThan(amt) {
				return false, fmt.Errorf(
					"not enough %v (%v) to meet demand (%v): %w",
					assetName,
					have[policyId][assetName].String(),
					amt,
					ErrInsufficientFunds,
				)
			}
		}
	}
	return true, nil
}

// A should be strictly less than B
// meaning for every asset in A, the amount of asset in B should be greater
// meaning loop over every A asset, because if there's an asset in a that's not in b, we need to fail
// but if there's an asset in b that's not in a, that's fine
func LessThanOrEqual(a, b Value) bool {
	for policy, policyMap := range a {
		for asset, aAmt := range policyMap {
			bAmt := num.Uint64(0)
			bAssets, ok := b[policy]
			if ok {
				bAmt = bAssets[asset]
			}
			if aAmt.GreaterThan(bAmt) {
				return false
			}
		}
	}

	return true
}

// A should be strictly greater than B
// meaning for every asset in B, the amount of asset in A should be greater
// meaning loop over b, because if there's an asset in b that's not in a, we need to fail
// but if there's an asset in a that's not in b, that's fine
func GreaterThanOrEqual(a, b Value) bool {
	for policy, policyMap := range b {
		for asset, bAmt := range policyMap {
			aAmt := num.Uint64(0)
			aAssets, ok := a[policy]
			if ok {
				aAmt = aAssets[asset]
			}
			if aAmt.LessThan(bAmt) {
				return false
			}
		}
	}

	return true
}

func Equal(a, b Value) bool {
	policies := map[string]bool{}
	for policy := range a {
		policies[policy] = true
	}
	for policy := range b {
		policies[policy] = true
	}

	for policy := range policies {
		aAssets, okA := a[policy]
		bAssets, okB := b[policy]
		assets := map[string]bool{}
		for asset := range aAssets {
			assets[asset] = true
		}
		for asset := range bAssets {
			assets[asset] = true
		}
		for asset := range assets {
			aAmt := num.Uint64(0)
			bAmt := num.Uint64(0)
			if okA {
				aAmt = aAssets[asset]
			}
			if okB {
				bAmt = bAssets[asset]
			}
			if !aAmt.Equal(bAmt) {
				return false
			}
		}
	}

	return true
}

func (v *Value) AddAsset(coins ...Coin) {
	// As a courtesy, initialize Value if necessary.
	if *v == nil {
		*v = Value{}
	}

	for _, coin := range coins {
		policy := coin.AssetId.PolicyID()
		asset := coin.AssetId.AssetName()
		if _, ok := (*v)[policy]; !ok {
			(*v)[policy] = map[string]num.Int{}
		}
		(*v)[policy][asset] = (*v)[policy][asset].Add(coin.Amount)
	}
}

func (v Value) AdaLovelace() num.Int {
	return v.AssetAmount(AdaAssetID)
}

func (v Value) AssetAmount(asset AssetID) num.Int {
	if nested, ok := v[asset.PolicyID()]; ok {
		return nested[asset.AssetName()]
	}
	return num.Int64(0)
}

func (v Value) AssetsExceptAda() Value {
	policies := Value{}
	for policy, tokenMap := range v {
		if policy == AdaPolicy {
			continue
		}
		policies[policy] = map[string]num.Int{}
		for token, quantity := range tokenMap {
			policies[policy][token] = quantity
		}
	}
	return policies
}

func (v Value) AssetsExceptAdaCount() uint32 {
	var cnt uint32 = 0
	for policy, tokenMap := range v {
		if policy == AdaPolicy {
			continue
		}
		cnt += uint32(len(tokenMap))
	}
	return cnt
}

func (v Value) IsAdaPresent() bool {
	if v[AdaPolicy] != nil {
		if v[AdaPolicy][AdaAsset].GreaterThan(num.Uint64(0)) {
			return true
		}
	}

	return false
}

type Coin struct {
	AssetId AssetID
	Amount  num.Int
}

func CreateAdaCoin(amt num.Int) Coin {
	return Coin{AssetId: AdaAssetID, Amount: amt}
}

func ValueFromCoins(coins ...Coin) Value {
	value := Value{}
	value.AddAsset(coins...)
	return value
}

func CreateAdaValue(amt int64) Value {
	value := Value{}
	value.AddAsset(CreateAdaCoin(num.Int64(amt)))
	return value
}
