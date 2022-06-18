package app

import (
	"encoding/hex"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/blake2b"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	mockUtils "rosen-bridge/tss/mocks"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"testing"
)

/*	TestRosenTss_SetMetadata
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are models.MetaData used as test arguments.
	Dependencies:
	-
*/
func TestRosenTss_SetMetadata(t *testing.T) {
	// setting fake peer home and creating files and folders
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	_, err = exec.Command("cp", "../mocks/_config_fixtures/config.json", "/tmp/.rosenTss/eddsa/config.json").Output()
	if err != nil {
		t.Error(err)
	}

	// cleaning up after each test case called
	t.Cleanup(func() {
		_, err = exec.Command("rm", "-rf", peerHome).Output()
		if err != nil {
			t.Error(err)
		}
	})

	tests := []struct {
		name string
		meta models.MetaData
	}{
		{
			name: "set meta data, rosenTss metaData should be equal to given one",
			meta: models.MetaData{
				Threshold:  2,
				PeersCount: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			app := rosenTss{
				peerHome: peerHome,
			}
			err := app.SetMetaData()
			if err != nil {
				t.Error(err)
			}

			assert.Equal(t, app.metaData, tt.meta)
		})
	}
}

/*	TestRosenTss_GetMetaData
	TestCases:
	testing message controller, there is 1 testcase.
	test for GetMetaData rosenTss metaData, It should be correct
	there are models.TssData used as test arguments.
	Dependencies:
	-
*/
func TestRosenTss_GetMetaData(t *testing.T) {
	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
	}

	metaData := app.GetMetaData()

	assert.Equal(t, app.metaData, metaData)
}

/*	TestRosenTss_GetStorage
	TestCases:
	testing message controller, there is 1 testcases.
	test for returning GetStorage object, It should be correct
	Dependencies:
	-
*/
func TestRosenTss_GetStorage(t *testing.T) {
	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		storage:    storage,
		connection: conn,
	}

	assert.Equal(t, app.storage, app.GetStorage())
}

/*	TestRosenTss_GetConnection
	TestCases:
	testing message controller, there is 1 testcase.
	test for GetConnection object, It should be correct
	Dependencies:
	-
*/
func TestRosenTss_GetConnection(t *testing.T) {
	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		storage:    storage,
		connection: conn,
	}

	assert.Equal(t, app.connection, app.GetConnection())
}

/*	TestRosenTss_GetPeerHome
	TestCases:
	testing message controller, there are 2 testcases.
	test for GetPeerHome, It should be equal to given one.
	Dependencies:
	-
*/
func TestRosenTss_GetPeerHome(t *testing.T) {
	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   ".rosenTss",
	}

	home := app.GetPeerHome()

	assert.Equal(t, app.peerHome, home)
}

/*	TestRosenTss_SetPeerHome
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	-
*/
func TestRosenTss_SetPeerHome(t *testing.T) {

	// get user home dir
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
	// creating absolute path from relative path
	absHomeAddress, err := filepath.Abs("./.rosenTss")
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name        string
		homeAddress string
		expected    string
	}{
		{
			name:        "relative home address, should be equal to expected",
			homeAddress: "./.rosenTss",
			expected:    absHomeAddress,
		},
		{
			name:        "user home address, should be equal to expected",
			homeAddress: "~/.rosenTss",
			expected:    fmt.Sprintf("%s/.rosenTss", userHome),
		},
	}

	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := rosenTss{
				metaData: models.MetaData{
					Threshold:  2,
					PeersCount: 3,
				},
				storage:    storage,
				connection: conn,
			}
			err := app.SetPeerHome(tt.homeAddress)
			if err != nil {
				t.Error(err)
			}

			assert.Equal(t, app.peerHome, tt.expected)
			_, err = exec.Command("rm", "-rf", tt.expected).Output()
			if err != nil {
				t.Error(err)
			}
		})
	}
}

/*	TestRosenTss_NewMessage
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are  models.GossipMessage used as test arguments.
	Dependencies:
	- tss.PartyId
*/
func TestRosenTss_NewMessage(t *testing.T) {

	// creating ner party id
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}

	message := fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromSign")

	tests := []struct {
		name          string
		gossipMessage models.GossipMessage
	}{
		{
			name: "creating gossip message from given data, the result must be correct",
			gossipMessage: models.GossipMessage{
				Message:    message,
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := rosenTss{}
			newGossipMessage := app.NewMessage(tt.gossipMessage.ReceiverId, tt.gossipMessage.SenderId,
				tt.gossipMessage.Message, tt.gossipMessage.MessageId, tt.gossipMessage.Name)

			assert.Equal(t, newGossipMessage, tt.gossipMessage)
		})
	}
}

/*	TestRosenTss_MessageHandler
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, receiverId used as test arguments.
	Dependencies:
	- storage function
	- network struct
*/
func TestRosenTss_MessageHandler(t *testing.T) {

	// using mocked connection and storage
	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		ChannelMap: make(map[string]chan models.Message),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   ".rosenTss",
	}
	messageCh := make(chan models.Message, 100)
	channelMap := make(map[string]chan models.Message)
	channelMap["ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1"] = messageCh

	tests := []struct {
		name       string
		channelMap map[string]chan models.Message
		message    models.Message
	}{
		{
			name: "the channel with messageId is exist in the channel map, there must be not error",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Crypto:     "eddsa",
					Name:       "partyId",
				},
			},
			channelMap: channelMap,
		},
		{
			name: "the channel with messageId is not exist in the channel map, there must be no error",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
					MessageId:  "aad5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Crypto:     "eddsa",
					Name:       "partyId",
				},
			},
			channelMap: channelMap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			app.MessageHandler(tt.message)

			msg := <-app.ChannelMap[tt.message.Message.MessageId]
			assert.Equal(t, msg, tt.message)
		})
	}
}

/*	TestRosenTss_StartNewSign
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage.LoadEDDSAKeygen function
	- network struct
*/
func TestRosenTss_StartNewSign(t *testing.T) {
	// creating peer home folder and files
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	_, err = exec.Command("cp", "../mocks/_config_fixtures/config.json", "/tmp/.rosenTss/eddsa/config.json").Output()
	if err != nil {
		t.Error(err)
	}

	// using mocked structs and functions
	storage := mockedStorage.NewStorage(t)
	storage.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(
		eddsaKeygen.LocalPartySaveData{}, nil, fmt.Errorf("successful"))
	conn := mockedNetwork.NewConnection(t)

	app := rosenTss{
		ChannelMap: make(map[string]chan models.Message),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   peerHome,
	}

	// creating fake channels and sign data
	messageCh := make(chan models.Message, 100)
	channelMap := make(map[string]chan models.Message)
	channelMap["ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1"] = messageCh
	message := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := hex.DecodeString(message.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := hex.EncodeToString(signDataBytes[:])

	tests := []struct {
		name       string
		channelMap map[string]chan models.Message
		messageId  string
	}{
		{
			name:       "there is an channel map to messageId in channel map",
			messageId:  messageId,
			channelMap: channelMap,
		},
		{
			name:       "there is no channel map to messageId in channel map",
			messageId:  "aad5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			channelMap: channelMap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := app.StartNewSign(message)
			if err != nil && err.Error() != "successful" {
				t.Error(err)
			}
		})
	}
}
