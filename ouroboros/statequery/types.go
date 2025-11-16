package statequery

import (
	"math/big"
)

type EraStart struct {
	Time  EraSeconds `json:"time"`
	Slot  big.Int    `json:"slot"`
	Epoch big.Int    `json:"epoch"`
}

type EraSeconds struct {
	Seconds big.Int `json:"seconds"`
}

type EraMilliseconds struct {
	Milliseconds big.Int `json:"milliseconds"`
}
