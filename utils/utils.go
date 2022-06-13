package utils

import (
	"crypto/elliptic"
	"crypto/rand"
	"github.com/btcsuite/btcutil/base58"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"golang.org/x/crypto/blake2b"
	"math/big"
)

// GenerateECDSAKey generates a new ECDSA key pair
func GenerateECDSAKey() ([]byte, *big.Int, *big.Int, error) {
	return elliptic.GenerateKey(elliptic.P256(), rand.Reader)
}

// GetPKFromECDSAPub returns the public key from an ECDSA public key
func GetPKFromECDSAPub(x *big.Int, y *big.Int) []byte {
	return elliptic.MarshalCompressed(elliptic.P256(), x, y)
}

// GetErgoAddressFromPK returns the Ergo address from a public key
func GetErgoAddressFromPK(pk []byte, testNet bool) string {
	address := []byte{1}
	var NetworkType byte
	const P2pkType = 1
	if testNet {
		NetworkType = 16
	} else {
		NetworkType = 0
	}
	prefixByte := make([]byte, 0)
	prefixByte = append(prefixByte, NetworkType+P2pkType)
	address = append(prefixByte, pk...)
	checksum := blake2b.Sum256(address)
	address = append(address, checksum[:32]...)
	return base58.Encode(address[:38])
}

// GenerateEDDSAKey generates a new EDDSA key pair.
func GenerateEDDSAKey() ([]byte, *big.Int, *big.Int, error) {
	return edwards.GenerateKey(rand.Reader)
}

// GetPKFromEDDSAPub returns the public key Serialized from an EDDSA public key.
func GetPKFromEDDSAPub(x *big.Int, y *big.Int) []byte {
	return edwards.NewPublicKey(x, y).Serialize()
}
