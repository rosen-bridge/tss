package utils

import (
	"crypto/elliptic"
	"crypto/rand"

	"github.com/binance-chain/tss-lib/tss"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/mr-tron/base58"
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

// GenerateEDDSAKey generates a new EDDSA key pair.
func GenerateEDDSAKey() ([]byte, *big.Int, *big.Int, error) {
	return edwards.GenerateKey(rand.Reader)
}

// GetPKFromEDDSAPub returns the public key Serialized from an EDDSA public key.
func GetPKFromEDDSAPub(x *big.Int, y *big.Int) []byte {
	return edwards.NewPublicKey(x, y).Serialize()
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

// IsPartyExist check if partyId exist in partyIds list or not
func IsPartyExist(newPartyId *tss.PartyID, partyIds tss.SortedPartyIDs) bool {
	for _, partyId := range partyIds {
		if partyId.Id == newPartyId.Id {
			return true
		}
	}
	return false
}

func HashToInt(hash []byte) *big.Int {
	c := elliptic.P256()
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}
