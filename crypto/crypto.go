package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
)

// 공개키 → 바이트 변환 함수들
func PublicKeyToUncompressedBytes(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(pub.Curve, pub.X, pub.Y)
}

// 바이트 → 공개키 변환 함수들
func UncompressedBytesToPublicKey(curve elliptic.Curve, X_byte []byte, Y_byte []byte) *ecdsa.PublicKey {
	X, Y := new(big.Int), new(big.Int)
	X.SetBytes(X_byte)
	Y.SetBytes(Y_byte)

	return &ecdsa.PublicKey{

		Curve: curve,
		X:     X,
		Y:     Y,
	}
}
