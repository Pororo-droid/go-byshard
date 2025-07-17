package message

import (
	"Pororo-droid/go-byshard/types"
	"time"
)

type Preprepare struct {
	Type     string
	View     types.View
	Sequence types.Sequence
	// PublicKey ecdsa.PublicKey
	PublicKey_X []byte
	PublicKey_Y []byte
	Request     Request
	Digest      types.ECDSASignature
	Timestamp   time.Time
}

type Prepare struct {
	Type        string
	View        types.View
	Sequence    types.Sequence
	PublicKey_X []byte
	PublicKey_Y []byte
	Preprepare  Preprepare
	Digest      types.ECDSASignature
	Timestamp   time.Time
}

type Commit struct {
	Type        string
	View        types.View
	Sequence    types.Sequence
	PublicKey_X []byte
	PublicKey_Y []byte
	PrepareList []*Prepare
	Digest      types.ECDSASignature
	Timestamp   time.Time
}
