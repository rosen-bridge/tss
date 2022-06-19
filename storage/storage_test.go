package storage

import (
	"encoding/hex"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
	"math/big"
	"os"
	"os/exec"
	"rosen-bridge/tss/models"
	"testing"
)

/*	TestEDDSA_Init
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	-
*/
func TestStorage_WriteData(t *testing.T) {
	// creating fake data for write
	message := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := hex.DecodeString(message.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())

	topicName := hex.EncodeToString(signDataBytes[:])
	data := models.MetaData{
		Threshold:  3,
		PeersCount: 4,
	}

	peerHome := "/tmp/.rosenTss"

	t.Cleanup(func() {
		_, err := exec.Command("rm", "-rf", peerHome).Output()
		if err != nil {
			t.Error(err)
		}
	})

	tests := []struct {
		name      string
		data      interface{}
		peerHome  string
		topicName string
	}{
		{
			name:      "write test data, there must be no error(the file must be created)",
			data:      data,
			peerHome:  peerHome,
			topicName: topicName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := storage{}
			err := s.WriteData(tt.data, tt.peerHome, tt.topicName, "test.txt", "eddsa")
			if err != nil {
				t.Error(err)
			}
		})
	}

}

/*	TestEDDSA_Init
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	-
*/
func TestStorage_LoadEDDSAKeygen(t *testing.T) {
	// creating peerHome
	peerHome := "/tmp/.rosenTss"
	path := fmt.Sprintf("%s/%s", peerHome, "eddsa")

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		t.Error(err)
	}

	_, err := exec.Command("cp", "../mocks/_eddsa_keygen_fixtures/keygen_data_00.json", "/tmp/.rosenTss/eddsa/keygen_data.json").Output()
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name     string
		peerHome string
	}{
		{
			name:     "load eddsa keygen data from home address, the result must have correct data",
			peerHome: peerHome,
		},
	}

	t.Cleanup(func() {
		_, err = exec.Command("rm", "-rf", peerHome).Output()
		if err != nil {
			t.Error(err)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := storage{}
			keygen, _, err := s.LoadEDDSAKeygen(tt.peerHome)
			if err != nil {
				t.Error(err)
			}
			assert.NotEqual(t, keygen, eddsaKeygen.LocalPartySaveData{})
		})
	}

}