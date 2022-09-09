package utils

import (
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"rosen-bridge/tss/models"

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

func GetAbsoluteAddress(address string) (string, error) {
	var absAddress string
	switch address[0:1] {
	case ".":
		addr, err := filepath.Abs(address)
		if err != nil {
			return "", err
		}
		absAddress = addr
	case "~":
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		addr := filepath.Join(userHome, address[1:])
		absAddress = addr
	case "/":
		absAddress = address
	default:
		return "", fmt.Errorf("wrong address format: %s", address)
	}
	return absAddress, nil
}

// InitConfig reads in config file and ENV variables if set.
func InitConfig(configFile string) (models.Config, error) {
	// Search config in home directory with name "default" (without extension).
	viper.SetConfigFile(configFile)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	if err != nil {
		return models.Config{}, fmt.Errorf("error using config file: %s", err.Error())
	}
	conf := models.Config{}
	err = viper.Unmarshal(&conf)
	if err != nil {
		return models.Config{}, fmt.Errorf("error Unmarshalling config file: %s", err.Error())
	}
	return conf, nil
}
