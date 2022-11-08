package app

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/blake2b"
	eddsaKeygenLocal "rosen-bridge/tss/app/keygen/eddsa"
	mockUtils "rosen-bridge/tss/mocks"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/utils"
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
	t.Cleanup(
		func() {
			_, err = exec.Command("rm", "-rf", peerHome).Output()
			if err != nil {
				t.Error(err)
			}
		},
	)

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

	logging, _ = mockUtils.InitLog("tss")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				app := rosenTss{
					peerHome: peerHome,
				}
				err := app.SetMetaData("eddsa")
				if err != nil {
					t.Error(err)
				}

				assert.Equal(t, app.metaData, tt.meta)
			},
		)
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
	logging, _ = mockUtils.InitLog("tss")

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
	logging, _ = mockUtils.InitLog("tss")
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
	logging, _ = mockUtils.InitLog("tss")
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
		peerHome:   "/tmp/.rosenTss",
	}
	logging, _ = mockUtils.InitLog("tss")
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
			homeAddress: "~/tmp/.rosenTss",
			expected:    fmt.Sprintf("%s/tmp/.rosenTss", userHome),
		},
		{
			name:        "complete home address, should be equal to expected",
			homeAddress: "/tmp/.rosenTss",
			expected:    "/tmp/.rosenTss",
		},
	}

	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
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
			},
		)
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
	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := rosenTss{}
				newGossipMessage := app.NewMessage(
					tt.gossipMessage.ReceiverId, tt.gossipMessage.SenderId,
					tt.gossipMessage.Message, tt.gossipMessage.MessageId, tt.gossipMessage.Name,
				)

				assert.Equal(t, newGossipMessage, tt.gossipMessage)
			},
		)
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
		ChannelMap: make(map[string]chan models.GossipMessage),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   "/tmp/.rosenTss",
		Config: models.Config{
			MessageTimeout: 5,
		},
	}
	messageCh := make(chan models.GossipMessage, 100)
	channelMap := make(map[string]chan models.GossipMessage)
	channelMap["ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1"] = messageCh

	gossipMessage := models.GossipMessage{
		Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:   "cahj2pgs4eqvn1eo1tp0",
		ReceiverId: "",
		Name:       "partyId",
	}
	marshal, err := json.Marshal(&gossipMessage)
	if err != nil {
		return
	}

	tests := []struct {
		name       string
		channelMap map[string]chan models.GossipMessage
		message    models.Message
		wantErr    bool
	}{
		{
			name: "the channel with messageId is exist in the channel map, there must be no error",
			message: models.Message{
				Topic:   "tss",
				Message: string(marshal),
			},
			channelMap: channelMap,
			wantErr:    false,
		},
		{
			name: "the channel with messageId is not exist in the channel map, there must be no error",
			message: models.Message{
				Topic:   "tss",
				Message: string(marshal),
			},
			channelMap: make(map[string]chan models.GossipMessage),
			wantErr:    true,
		},
	}
	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app.ChannelMap = tt.channelMap

				err = app.MessageHandler(tt.message)
				if err != nil {
					if !tt.wantErr {
						t.Errorf("MessageHandler error: %v", err)
					}
				} else {

					msg := <-app.ChannelMap[gossipMessage.MessageId]
					marshal, err := json.Marshal(&msg)
					if err != nil {
						return
					}
					assert.Equal(t, string(marshal), tt.message.Message)
				}
			},
		)
	}
}

/*	TestRosenTss_deleteInstance
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage LoadPrivate, WriteData, LoadEDDSAKeygen function
	- network Publish function
*/
func TestRosenTss_deleteInstance(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}

	messageCh := make(chan models.GossipMessage, 100)
	channelMap := make(map[string]chan models.GossipMessage)
	channelId := "regroup"
	channelMap[channelId] = messageCh

	tests := []struct {
		name       string
		channelMap map[string]chan models.GossipMessage
		message    models.RegroupMessage
	}{
		{
			name:       "deleting channel and operation",
			channelMap: channelMap,
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    0,
				Crypto:       "eddsa",
			},
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := rosenTss{
					ChannelMap: make(map[string]chan models.GossipMessage),
					peerHome:   peerHome,
				}
				app.ChannelMap = tt.channelMap
				EDDSAOperation := eddsaKeygenLocal.NewKeygenEDDSAOperation(models.KeygenMessage{})
				app.operations = append(app.operations, EDDSAOperation)

				app.deleteInstance(channelId, EDDSAOperation.GetClassName())
				if _, ok := app.ChannelMap[channelId]; ok {
					t.Errorf("deleteInstance error: channel still exist")
				}
				for _, operation := range app.operations {
					if operation.GetClassName() == EDDSAOperation.GetClassName() {
						t.Errorf("deleteInstance error: operation still exist")
					}
				}
			},
		)
	}
}

/*	TestRosenTss_GetPublicKey
	TestCases:
	testing message controller, there are 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage LoadECDSAKeygen, LoadEDDSAKeygen function
*/
func TestRosenTss_GetPublicKey(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}

	ecdsaPK, err := mockUtils.GetEcdsaPK()
	if err != nil {
		t.Error(err)
	}
	eddsaPK, err := mockUtils.GetEddsaPK()
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name      string
		crypto    string
		expected  string
		wantErr   bool
		appConfig func() rosenTss
	}{
		{
			name:     "ecdsa key",
			crypto:   "ecdsa",
			expected: ecdsaPK,
			wantErr:  false,
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				data, id, err := mockUtils.LoadECDSAKeygenFixture(0)
				if err != nil {
					t.Errorf("LoadECDSAKeygenFixture error = %v", err)
				}
				store.On("LoadECDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				return rosenTss{
					peerHome: peerHome,
					storage:  store,
				}
			},
		},
		{
			name:     "eddsa key",
			crypto:   "eddsa",
			expected: eddsaPK,
			wantErr:  false,
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				data, id, err := mockUtils.LoadEDDSAKeygenFixture(0)
				if err != nil {
					t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
				}
				store.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				return rosenTss{
					peerHome: peerHome,
					storage:  store,
				}
			},
		},
		{
			name:     "wrong crypto",
			crypto:   "",
			expected: "",
			wantErr:  true,
			appConfig: func() rosenTss {
				return rosenTss{
					peerHome: peerHome,
				}
			},
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	// cleaning up after each test case called
	t.Cleanup(
		func() {
			_, err = exec.Command("rm", "-rf", peerHome).Output()
			if err != nil {
				t.Error(err)
			}
		},
	)

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				key, err := app.GetPublicKey(tt.crypto)
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
				assert.Equal(t, key, tt.expected)
			},
		)
	}
}

/*	TestRosenTss_SetPrivate
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are models.Private used as test arguments.
	Dependencies:
	- storage WriteData function
*/
func TestRosenTss_SetPrivate(t *testing.T) {
	// setting fake peer home and creating files and folders
	peerHome := "/tmp/.rosenTss"

	tests := []struct {
		name    string
		private models.Private
	}{
		{
			name: "set private, there must be no error",
			private: models.Private{
				Private: "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d",
				Crypto:  "ecdsa",
			},
		},
	}

	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				app := rosenTss{
					peerHome: peerHome,
					storage:  store,
				}
				err := app.SetPrivate(tt.private)
				if err != nil {
					t.Error(err)
				}
			},
		)
	}
}

/*	TestRosenTss_GetPrivate
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are models.Private used as test arguments.
	Dependencies:
	- storage LoadPrivate function
*/
func TestRosenTss_GetPrivate(t *testing.T) {
	// setting fake peer home and creating files and folders
	peerHome := "/tmp/.rosenTss"

	tests := []struct {
		name    string
		private models.Private
	}{
		{
			name: "set private, there must be no error",
			private: models.Private{
				Private: "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d",
				Crypto:  "ecdsa",
			},
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				store := mockedStorage.NewStorage(t)
				store.On(
					"LoadPrivate", mock.AnythingOfType("string"),
					mock.AnythingOfType("string"),
				).Return(tt.private.Private, nil)
				app := rosenTss{
					peerHome: peerHome,
					storage:  store,
				}
				private := app.GetPrivate(tt.private.Crypto)
				assert.Equal(t, private, tt.private.Private)
			},
		)
	}
}

func TestRosenTss_SetP2pId(t *testing.T) {
	// setting fake peer home and creating files and folders
	peerHome := "/tmp/.rosenTss"

	tests := []struct {
		name       string
		expectedId string
		connConfig func() network.Connection
	}{
		{
			name:       "set p2p id, there must be no error",
			expectedId: "3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD",
			connConfig: func() network.Connection {
				conn := mockedNetwork.NewConnection(t)
				conn.On("GetPeerId").Return("3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD", nil)
				return conn
			},
		},
		{
			name:       "set p2p id, there must be an error",
			expectedId: "3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD",
			connConfig: func() network.Connection {
				conn := mockedNetwork.NewConnection(t)
				conn.On("GetPeerId").Return("", fmt.Errorf("connection error"))
				return conn
			},
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				app := rosenTss{
					peerHome:   peerHome,
					connection: tt.connConfig(),
				}
				err := app.SetP2pId()
				if err != nil && err.Error() != "connection error" {
					t.Error(err)
				}
				if app.P2pId != tt.expectedId && err.Error() != "connection error" {
					t.Error("wrong p2p id")
				}
			},
		)
	}
}

//-------------------------------ECDSA-------------------------------
/*	TestRosenTss_StartNewSign_ECDSA
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage.LoadECDSAKeygen function
	- network struct
*/
func TestRosenTss_StartNewSign_ECDSA(t *testing.T) {

	// creating peer home folder and files
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/ecdsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	_, err = exec.Command("cp", "../mocks/_config_fixtures/config.json", "/tmp/.rosenTss/ecdsa/config.json").Output()
	if err != nil {
		t.Error(err)
	}

	savedData, pID, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		return
	}
	// using mocked structs and functions
	storage := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)

	// creating fake channels and sign data
	message := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "ecdsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(message.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "ecdsa", hex.EncodeToString(signDataBytes[:]))

	messageCh := make(chan models.GossipMessage, 100)
	channelMap := make(map[string]chan models.GossipMessage)
	channelMapWithoutMessageId := make(map[string]chan models.GossipMessage)
	channelMapWithoutMessageId["no sign"] = messageCh
	channelMap[messageId] = messageCh

	app := rosenTss{
		ChannelMap: make(map[string]chan models.GossipMessage),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   peerHome,
		Config: models.Config{
			OperationTimeout:          60,
			MessageTimeout:            5,
			LeastProcessRemainingTime: 50,
			SetupBroadcastInterval:    10,
			SignStartTimeTracker:      1,
			TurnDuration:              60,
		},
	}

	tests := []struct {
		name       string
		channelMap map[string]chan models.GossipMessage
		messageId  string
		wantErr    bool
		appConfig  func() rosenTss
		message    models.SignMessage
	}{
		{
			name:       "there is an channel map to messageId in channel map",
			channelMap: channelMap,
			wantErr:    true,
			appConfig: func() rosenTss {
				return app
			},
			message: message,
		},
		{
			name:       "there is an channel map to messageId in channel map early stop timeout",
			channelMap: channelMap,
			wantErr:    true,
			appConfig: func() rosenTss {
				return app
			},
			message: message,
		},
		{
			name:       "there is an channel map to messageId in channel map early stop timeout",
			channelMap: channelMap,
			wantErr:    true,
			appConfig: func() rosenTss {
				return app
			},
			message: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "secp256",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
		},
		{
			name:       "there is no channel map to messageId in channel map",
			channelMap: channelMapWithoutMessageId,
			wantErr:    false,
			appConfig: func() rosenTss {
				storage := mockedStorage.NewStorage(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				storage.On("LoadECDSAKeygen", mock.AnythingOfType("string")).Return(
					savedData, pID, nil,
				)
				app.connection = conn
				app.storage = storage
				return app

			},
			message: message,
		},
	}
	logging, _ = mockUtils.InitLog("tss")
	//wg := new(sync.WaitGroup)

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				app.ChannelMap = tt.channelMap
				//wg.Add(1)
				err := app.StartNewSign(tt.message)
				//wg.Wait()
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
			},
		)
	}
}

/*	TestRosenTss_StartNewKeygen_ECDSA
	TestCases:
	testing message controller, there are 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage LoadPrivate, WriteData function
	- network Publish function
*/
func TestRosenTss_StartNewKeygen_ECDSA(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/ecdsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}

	priv, _, _, err := utils.GenerateECDSAKey()
	if err != nil {
		t.Fatal(err)
	}

	storage := mockedStorage.NewStorage(t)
	storage.On(
		"WriteData",
		mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Return(nil)
	storage.On(
		"LoadPrivate", mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Return(hex.EncodeToString(priv), nil)

	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := rosenTss{
		ChannelMap: make(map[string]chan models.GossipMessage),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   peerHome,
	}
	messageCh := make(chan models.GossipMessage, 100)
	channelMapWithKeygen := make(map[string]chan models.GossipMessage)
	channelMapWithoutKeygen := make(map[string]chan models.GossipMessage)
	channelMapWithKeygen["keygen"] = messageCh
	channelMapWithoutKeygen["no keygen"] = messageCh
	message := models.KeygenMessage{
		Threshold:  2,
		PeersCount: 3,
		Crypto:     "ecdsa",
	}

	tests := []struct {
		name       string
		channelMap map[string]chan models.GossipMessage
		wantErr    bool
	}{
		{
			name:       "with channel id",
			channelMap: channelMapWithKeygen,
			wantErr:    true,
		},
		{
			name:       "without channel id",
			channelMap: channelMapWithoutKeygen,
			wantErr:    false,
		},
		{
			name:       "error in loop",
			channelMap: channelMapWithKeygen,
			wantErr:    true,
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app.ChannelMap = tt.channelMap
				if tt.name == "error in loop" {
					app.ChannelMap["keygen"] <- models.GossipMessage{
						Message:    "generate key",
						MessageId:  "keygen",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "keygen",
					}
				}
				err := app.StartNewKeygen(message)
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
			},
		)
	}
}

/*	TestRosenTss_StartNewRegroup_ECDSA
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage LoadPrivate, WriteData, LoadEDDSAKeygen function
	- network Publish function
*/
func TestRosenTss_StartNewRegroup_ECDSA(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/ecdsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}

	// reading ecdsaKeygen.LocalPartySaveData data from fixtures

	priv, _, _, err := utils.GenerateECDSAKey()
	if err != nil {
		t.Fatal(err)
	}

	messageCh := make(chan models.GossipMessage, 100)
	channelMapWithRegroup := make(map[string]chan models.GossipMessage)
	channelMapWithoutRegroup := make(map[string]chan models.GossipMessage)
	channelMapWithRegroup["ecdsaRegroup"] = messageCh
	channelMapWithoutRegroup["no regroup"] = messageCh

	tests := []struct {
		name      string
		message   models.RegroupMessage
		appConfig func() rosenTss
		wantErr   bool
	}{
		{
			name: "with channel id, peerStare 0",
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    0,
				Crypto:       "ecdsa",
			},
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				conn := mockedNetwork.NewConnection(t)
				return rosenTss{
					ChannelMap: channelMapWithRegroup,
					storage:    store,
					connection: conn,
					peerHome:   peerHome,
				}
			},
			wantErr: true,
		},
		{
			name: "without channel id, peerStare 1 with private",
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    1,
				Crypto:       "ecdsa",
			},
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				store.On(
					"LoadPrivate", mock.AnythingOfType("string"),
					mock.AnythingOfType("string"),
				).Return(hex.EncodeToString(priv), nil)

				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				return rosenTss{
					ChannelMap: channelMapWithoutRegroup,
					storage:    store,
					connection: conn,
					peerHome:   peerHome,
				}
			},
			wantErr: false,
		},
		{
			name: "without channel id, peerStare 1 without private",
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    1,
				Crypto:       "ecdsa",
			},
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				store.On(
					"WriteData",
					mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"),
					mock.AnythingOfType("string"),
				).Return(nil)
				store.On("LoadPrivate", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("", nil)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				return rosenTss{
					ChannelMap: make(map[string]chan models.GossipMessage),
					storage:    store,
					connection: conn,
					peerHome:   peerHome,
				}
			},
			wantErr: false,
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				t.Log(app.ChannelMap)
				err = app.StartNewRegroup(tt.message)
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
			},
		)
	}
}

//-------------------------------EDDSA-------------------------------

/*	TestRosenTss_StartNewSign_EDDSA
	TestCases:
	testing message controller, there are 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage.LoadEDDSAKeygen function
	- network struct
*/
func TestRosenTss_StartNewSign_EDDSA(t *testing.T) {
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
	saveData, pId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		return
	}
	// using mocked structs and functions
	storage := mockedStorage.NewStorage(t)

	conn := mockedNetwork.NewConnection(t)

	// creating fake channels and sign data
	message := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(message.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "eddsa", hex.EncodeToString(signDataBytes[:]))

	messageCh := make(chan models.GossipMessage, 100)
	channelMap := make(map[string]chan models.GossipMessage)
	channelMapWithoutMessageId := make(map[string]chan models.GossipMessage)
	channelMapWithoutMessageId["no sign"] = messageCh
	channelMap[messageId] = messageCh

	app := rosenTss{
		ChannelMap: make(map[string]chan models.GossipMessage),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   peerHome,
		Config: models.Config{
			OperationTimeout:          60,
			MessageTimeout:            5,
			LeastProcessRemainingTime: 50,
			SetupBroadcastInterval:    10,
			SignStartTimeTracker:      1,
			TurnDuration:              60,
		},
	}

	tests := []struct {
		name       string
		channelMap map[string]chan models.GossipMessage
		messageId  string
		wantErr    bool
		appConfig  func() rosenTss
	}{
		{
			name:       "there is an channel map to messageId in channel map",
			channelMap: channelMap,
			wantErr:    true,
			appConfig: func() rosenTss {
				return app
			},
		},
		{
			name:       "there is an channel map to messageId in channel map early stop with timeout",
			channelMap: channelMap,
			wantErr:    true,
			appConfig: func() rosenTss {
				return app
			},
		},
		{
			name:       "there is no channel map to messageId in channel map",
			channelMap: channelMapWithoutMessageId,
			wantErr:    false,
			appConfig: func() rosenTss {
				storage := mockedStorage.NewStorage(t)
				conn := mockedNetwork.NewConnection(t)
				storage.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(
					saveData, pId, nil,
				)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.connection = conn
				app.storage = storage
				return app
			},
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				app.ChannelMap = tt.channelMap
				err := app.StartNewSign(message)
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
			},
		)
	}
}

/*	TestRosenTss_StartNewKeygen_EDDSA
	TestCases:
	testing message controller, there are 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage LoadPrivate, WriteData function
	- network Publish function
*/
func TestRosenTss_StartNewKeygen_EDDSA(t *testing.T) {
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
	storage.On(
		"WriteData",
		mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Return(nil)
	storage.On(
		"LoadPrivate", mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Return(hex.EncodeToString(priv), nil)

	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := rosenTss{
		ChannelMap: make(map[string]chan models.GossipMessage),
		metaData: models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
		storage:    storage,
		connection: conn,
		peerHome:   peerHome,
	}
	messageCh := make(chan models.GossipMessage, 100)
	channelMapWithKeygen := make(map[string]chan models.GossipMessage)
	channelMapWithoutKeygen := make(map[string]chan models.GossipMessage)
	channelMapWithKeygen["keygen"] = messageCh
	channelMapWithoutKeygen["no keygen"] = messageCh
	message := models.KeygenMessage{
		Threshold:  2,
		PeersCount: 3,
		Crypto:     "eddsa",
	}

	tests := []struct {
		name       string
		channelMap map[string]chan models.GossipMessage
		wantErr    bool
	}{
		{
			name:       "with channel id",
			channelMap: channelMapWithKeygen,
			wantErr:    true,
		},
		{
			name:       "without channel id",
			channelMap: channelMapWithoutKeygen,
			wantErr:    false,
		},
		{
			name:       "error in loop",
			channelMap: channelMapWithKeygen,
			wantErr:    true,
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app.ChannelMap = tt.channelMap
				if tt.name == "error in loop" {
					app.ChannelMap["keygen"] <- models.GossipMessage{
						Message:    "generate key",
						MessageId:  "keygen",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "keygen",
					}
				}
				err := app.StartNewKeygen(message)
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
			},
		)
	}
}

/*	TestRosenTss_StartNewRegroup_EDDSA
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- storage LoadPrivate, WriteData, LoadEDDSAKeygen function
	- network Publish function
*/
func TestRosenTss_StartNewRegroup_EDDSA(t *testing.T) {
	peerHome := "/tmp/.rosenTss"
	err := os.MkdirAll(fmt.Sprintf("%s/eddsa", peerHome), os.ModePerm)
	if err != nil {
		t.Error(err)
	}

	priv, _, _, err := utils.GenerateEDDSAKey()
	if err != nil {
		t.Fatal(err)
	}

	messageCh := make(chan models.GossipMessage, 100)
	channelMapWithRegroup := make(map[string]chan models.GossipMessage)
	channelMapWithoutRegroup := make(map[string]chan models.GossipMessage)
	channelMapWithRegroup["eddsaRegroup"] = messageCh
	channelMapWithoutRegroup["no regroup"] = messageCh

	tests := []struct {
		name      string
		message   models.RegroupMessage
		appConfig func() rosenTss
		wantErr   bool
	}{
		{
			name: "with channel id, peerStare 0",
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    0,
				Crypto:       "eddsa",
			},
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				conn := mockedNetwork.NewConnection(t)

				return rosenTss{
					ChannelMap: channelMapWithRegroup,
					storage:    store,
					connection: conn,
					peerHome:   peerHome,
				}
			},
			wantErr: true,
		},
		{
			name: "without channel id, peerStare 1 with private",
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    1,
				Crypto:       "eddsa",
			},
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				store.On(
					"LoadPrivate", mock.AnythingOfType("string"),
					mock.AnythingOfType("string"),
				).Return(hex.EncodeToString(priv), nil)

				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				return rosenTss{
					ChannelMap: channelMapWithoutRegroup,
					storage:    store,
					connection: conn,
					peerHome:   peerHome,
				}
			},
			wantErr: false,
		},
		{
			name: "without channel id, peerStare 1 without private",
			message: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeersCount:   3,
				PeerState:    1,
				Crypto:       "eddsa",
			},
			appConfig: func() rosenTss {
				store := mockedStorage.NewStorage(t)
				store.On(
					"WriteData",
					mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"),
					mock.AnythingOfType("string"),
				).Return(nil)
				store.On("LoadPrivate", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("", nil)

				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				return rosenTss{
					ChannelMap: make(map[string]chan models.GossipMessage),
					storage:    store,
					connection: conn,
					peerHome:   peerHome,
				}
			},
			wantErr: false,
		},
	}

	logging, _ = mockUtils.InitLog("tss")

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()

				err = app.StartNewRegroup(tt.message)
				if err != nil && !tt.wantErr {
					t.Error(err)
				}
			},
		)
	}
}
