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
	"rosen-bridge/tss/utils"
	"testing"
)

func TestRosenTss_SetMetadata(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	_, err = exec.Command("cp", "../mocks/_config_fixtures/config.json", "/tmp/.rosenTss/eddsa/config.json").Output()
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name string
		meta models.MetaData
	}{
		{
			name: "relative home address",
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

func TestRosenTss_GetStorage(t *testing.T) {

	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		storage:    storage,
		connection: conn,
	}

	assert.Equal(t, app.storage, app.GetStorage())
}

func TestRosenTss_GetConnection(t *testing.T) {

	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	app := rosenTss{
		storage:    storage,
		connection: conn,
	}

	assert.Equal(t, app.connection, app.GetConnection())
}

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

func TestRosenTss_SetPeerHome(t *testing.T) {

	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
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
			name:        "relative home address",
			homeAddress: "./.rosenTss",
			expected:    absHomeAddress,
		},
		{
			name:        "user home address",
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

func TestRosenTss_NewMessage(t *testing.T) {

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
			name: "relative home address",
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

func TestRosenTss_MessageHandler(t *testing.T) {

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
			name: "with channel id",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyId",
				},
			},
			channelMap: channelMap,
		},
		{
			name: "without channel id",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
					MessageId:  "aad5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
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

func TestRosenTss_StartNewSign(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	_, err = exec.Command("cp", "../mocks/_config_fixtures/config.json", "/tmp/.rosenTss/eddsa/config.json").Output()
	if err != nil {
		t.Error(err)
	}

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
			name:       "with channel id",
			messageId:  messageId,
			channelMap: channelMap,
		},
		{
			name:       "without channel id",
			messageId:  "aad5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			channelMap: channelMap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.ChannelMap = tt.channelMap
			err := app.StartNewSign(message)
			if err != nil && err.Error() != "successful" {
				t.Error(err)
			}
		})
	}
}

func TestRosenTss_StartNewKeygen(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}

	priv, _, _, err := utils.GenerateEDDSAKey()
	if err != nil {
		t.Fatal(err)
	}

	storage := mockedStorage.NewStorage(t)
	storage.On("WriteData",
		mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	storage.On("LoadPrivate", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(hex.EncodeToString(priv), nil)

	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

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
	messageCh := make(chan models.Message, 100)
	channelMapWithKeygen := make(map[string]chan models.Message)
	channelMapWithoutKeygen := make(map[string]chan models.Message)
	channelMapWithKeygen["keygen"] = messageCh
	channelMapWithoutKeygen["no keygen"] = messageCh
	message := models.KeygenMessage{
		Threshold:  2,
		PeersCount: 3,
		Crypto:     "eddsa",
	}

	tests := []struct {
		name       string
		channelMap map[string]chan models.Message
	}{
		{
			name:       "with channel id",
			channelMap: channelMapWithKeygen,
		},
		{
			name:       "without channel id",
			channelMap: channelMapWithoutKeygen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.ChannelMap = tt.channelMap
			err := app.StartNewKeygen(message)
			if err != nil && err.Error() != "successful" {
				t.Error(err)
			}
		})
	}
}
