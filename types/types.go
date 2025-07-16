package types

import (
	"encoding/json"
	"math/big"
)

type View int
type Sequence int

type ECDSASignature [][]byte

type BigInt struct {
	big.Int
}

func (i *BigInt) UnmarshalJSON(b []byte) error {
	var val string
	err := json.Unmarshal(b, &val)
	if err != nil {
		return err
	}

	i.SetString(val, 10)

	return nil
}

func (sig *ECDSASignature) Unmarshal() (*BigInt, *BigInt) {
	signature := *sig
	var r, s BigInt
	r.SetString(string(signature[0]), 10)
	s.SetString(string(signature[1]), 10)
	return &r, &s
}
