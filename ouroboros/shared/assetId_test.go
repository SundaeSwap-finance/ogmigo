package shared

import (
	"regexp"
	"testing"

	"github.com/tj/assert"
)

func Test_AssetID(t *testing.T) {
	a1 := FromSeparate(
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		"4c51",
	)
	a2 := FromSeparate(
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		"",
	)
	a3 := FromSeparate("", "")
	a4 := AssetID("da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24.")
	a5 := FromSeparate(
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		"Eeyor3",
	)
	a6 := FromSeparate(
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		"e8ada6e5af9fe69685e4bab9",
	)
	re1 := regexp.MustCompile(`^[a-fA-F0-9]+\.[a-fA-F0-9]+$`)
	re2 := regexp.MustCompile(`^[Ee]+yo(r*)(3|E)$`)
	// a3 := FromSeparate("abcd", "1234")
	// a4 := FromSeparate("abcd", "")

	assert.EqualValues(
		t,
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24.4c51",
		a1.String(),
	)
	assert.EqualValues(
		t,
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		a2.String(),
	)
	assert.EqualValues(
		t,
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		a1.PolicyID(),
	)
	assert.EqualValues(
		t,
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		a2.PolicyID(),
	)
	assert.EqualValues(t, "4c51", a1.AssetName())
	assert.EqualValues(t, "", a2.AssetName())
	assert.EqualValues(
		t,
		true,
		a1.HasPolicyID(
			"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		),
	)
	assert.EqualValues(t, false, a1.HasPolicyID("da"))
	assert.EqualValues(t, false, a1.IsZero())
	assert.EqualValues(t, true, a1.HasAssetID(re1))
	assert.EqualValues(t, false, a2.HasAssetID(re1))
	assert.EqualValues(
		t,
		true,
		a2.HasPolicyID(
			"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		),
	)
	assert.EqualValues(t, false, a2.IsZero())
	assert.EqualValues(t, true, a3.IsZero())
	assert.EqualValues(
		t,
		"da8c30857834c6ae7203935b89278c532b3995245295456f993e1d24",
		a4.PolicyID(),
	)
	assert.EqualValues(t, "", a4.AssetName())
	assert.EqualValues(t, false, a4.HasAssetID(re1))
	regexArray1, found1 := a5.MatchAssetName(re2)
	ex1 := []string{"Eeyor3", "r", "3"}
	assert.EqualValues(t, ex1, regexArray1)
	assert.EqualValues(t, true, found1)
	utf8Data1, isUtf8Bool1 := a1.AssetNameUTF8()
	assert.EqualValues(t, "LQ", string(utf8Data1))
	assert.EqualValues(t, true, isUtf8Bool1)
	utf8Data2, isUtf8Bool2 := a6.AssetNameUTF8()
	assert.EqualValues(t, "警察斅亹", utf8Data2)
	assert.EqualValues(t, true, isUtf8Bool2)
}
