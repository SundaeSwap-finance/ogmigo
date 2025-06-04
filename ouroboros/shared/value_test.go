package shared

import (
	"fmt"
	"testing"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	"github.com/tj/assert"
)

func Test_AssetsExceptADA(t *testing.T) {
	v := Value{
		"ada": {
			"lovelace": num.Uint64(1),
		},
		"policy1": {
			"asset1": num.Uint64(1),
			"asset2": num.Uint64(2),
		},
		"policy2": {
			"asset3": num.Uint64(3),
			"asset4": num.Uint64(4),
		},
	}

	a := v.AssetsExceptAda()
	fmt.Printf("%v\n", a)
	assert.EqualValues(t, a, Value{
		"policy1": {
			"asset1": num.Uint64(1),
			"asset2": num.Uint64(2),
		},
		"policy2": {
			"asset3": num.Uint64(3),
			"asset4": num.Uint64(4),
		},
	})
}

func Test_Enough(t *testing.T) {
	have := Value{
		"ada": {
			"lovelace": num.Uint64(437041203),
		},
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24": {
			"4c51": num.Uint64(14310359231),
		},
	}
	want := Value{
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24": {
			"4c51": num.Uint64(1023291),
		},
		"25c5de5f5b286073c593edfd77b48abc7a48e5a4f3d4cd9d428ff935": {
			"55534454": num.Uint64(3449),
		},
	}
	ok, _ := Enough(have, want)
	assert.False(t, ok)
}

func Test_AddAsset(t *testing.T) {
	v1 := Value{
		"ada": {
			"lovelace": num.Uint64(1),
		},
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24": {
			"4c51": num.Uint64(14310359231),
		},
	}
	var v2 Value
	v2.AddAsset(
		CreateAdaCoin(num.Uint64(1)),
		Coin{
			AssetId: FromSeparate(
				"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
				"4c51",
			),
			Amount: num.Uint64(14310359231),
		},
	)
	var v3 Value
	v3.AddAsset(
		Coin{
			AssetId: FromSeparate(
				"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
				"4c51",
			),
			Amount: num.Uint64(14310359231),
		},
	)

	assert.EqualValues(t, v1, v2)
	assert.EqualValues(t, 1, v2.AssetsExceptAdaCount())
	assert.EqualValues(t, true, v2.IsAdaPresent())
	assert.EqualValues(t, num.Uint64(1), v2.AssetAmount(AdaAssetID))
	assert.EqualValues(
		t,
		num.Uint64(14310359231),
		v2.AssetAmount(
			FromSeparate(
				"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
				"4c51",
			),
		),
	)
	assert.EqualValues(
		t,
		num.Uint64(0),
		v2.AssetAmount(
			FromSeparate(
				"ea8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
				"4c51",
			),
		),
	)
	assert.EqualValues(
		t,
		num.Uint64(0),
		v2.AssetAmount(
			FromSeparate(
				"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
				"4c52",
			),
		),
	)
	assert.EqualValues(t, false, v3.IsAdaPresent())
}
