package crypto

import (
	"Pororo-droid/go-byshard/types"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
)

func ECDSASign(priv *ecdsa.PrivateKey, msg []byte) (types.ECDSASignature, error) {
	var r, s *big.Int
	var err error
	r, s, err = ecdsa.Sign(rand.Reader, priv, msg)
	if err != nil {
		return nil, err
	}
	sig := make([][]byte, 2)
	sig[0] = []byte(r.String())
	sig[1] = []byte(s.String())
	return sig, err
}

func ECDSAVerify(pub ecdsa.PublicKey, data []byte, sig types.ECDSASignature) (bool, error) {
	r, s := sig.Unmarshal()
	is_verified := ecdsa.Verify(&pub, data, &r.Int, &s.Int)
	return is_verified, nil
}
