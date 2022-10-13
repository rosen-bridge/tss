package eddsa

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/binance-chain/tss-lib/common"
	eddsaSign "github.com/binance-chain/tss-lib/eddsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/blake2b"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	mockUtils "rosen-bridge/tss/mocks"
	mockedInterface "rosen-bridge/tss/mocks/app/interface"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
)

/*	TestEDDSA_Init
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, receiverId used as test arguments.
	Dependencies:
	- localTssData
	- eddsaKeygen.LocalPartySaveData
	- storage.LoadEDDSAKeygen function
	- network.Publish function
	- rosenTss GetStorage, GetConnection, GetPeerHome, NewMessage functions
*/
func TestEDDSA_Init(t *testing.T) {

	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	// creating fake localTssData
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalEDDSATSSData error = %v", err)
	}

	// reading eddsaKeygen.LocalPartySaveData data from fixtures
	data, id, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	// using mock functions

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssData
		appConfig    func() _interface.RosenTss
	}{
		{
			name: "creating register message with localTssData, there must be no error",
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)

				return app
			},
			receiverId:   "",
			localTssData: localTssData,
		},
		{
			name: "creating register message without localTssData, there must be no error",
			appConfig: func() _interface.RosenTss {
				storage := mockedStorage.NewStorage(t)
				storage.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetStorage").Return(storage)
				app.On("GetConnection").Return(conn)
				app.On("GetPeerHome").Return(".rosenTss")

				return app
			},
			receiverId:   "cahj2pgs4eqvn1eo1tp0",
			localTssData: models.TssData{},
		},
	}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.appConfig()
			eddsaSignOp := operationEDDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := eddsaSignOp.Init(app, tt.receiverId)
			if err != nil {
				t.Errorf("Init failed: %v", err)
			}

		})
	}
}

/*	TestEDDSA_Loop
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.Message used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestEDDSA_Loop(t *testing.T) {
	// pre-test part, faking data and using mocks

	// reading eddsaKeygen.LocalPartySaveData from fixtures
	saveData, Id1, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	saveData2, Id2, err := mockUtils.LoadEDDSAKeygenFixture(2)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	_, Id3, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	// creating localTssData and new partyId
	localTssData := models.TssData{
		PartyID: Id1,
	}
	newPartyId := Id2

	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), Id3))
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID))
	signDataBytes, _ := utils.Decoder("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(signDataBytes)

	// creating new tss party for eddsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))
	party := eddsaSign.NewLocalParty(signData, localTssData.Params, saveData, outCh, endCh)

	registerMessage := models.Register{
		Id:        newPartyId.Id,
		Moniker:   newPartyId.Moniker,
		Key:       newPartyId.KeyInt().String(),
		Timestamp: time.Now().Unix() / 60,
		NoAnswer:  false,
	}
	marshal, err := json.Marshal(registerMessage)
	if err != nil {
		t.Errorf("registerMessage error = %v", err)
	}

	partyMessage := models.PartyMessage{
		Message:     marshal,
		IsBroadcast: true,
		GetFrom:     newPartyId,
		To:          []*tss.PartyID{localTssData.PartyID},
	}

	partyMessageBytes, err := json.Marshal(partyMessage)
	if err != nil {
		t.Error("failed to marshal message", err)
	}

	messageBytes := blake2b.Sum256(signData.Bytes())
	setupMessage := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     localTssData.PartyIds,
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId,
	}
	setupMessageMarshaled, err := json.Marshal(setupMessage)
	if err != nil {
		t.Errorf("setupMessageMarshaled error = %v", err)
	}

	// test cases
	tests := []struct {
		name      string
		expected  string
		message   models.GossipMessage
		AppConfig func() _interface.RosenTss
	}{
		{
			name:     "register",
			expected: "handling incoming register message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    string(marshal),
				MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       "register",
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				return app
			},
		},
		{
			name:     "setup",
			expected: "handling incoming setup message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    string(setupMessageMarshaled),
				MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       "setup",
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				return app
			},
		},
		{
			name:     "partyMsg",
			expected: "handling incoming partyMsg message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    hex.EncodeToString(partyMessageBytes),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       "partyMsg",
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				localTssData.Party = party
				return app
			},
		},
		{
			name:     "start sign with party",
			expected: "handling incoming sign message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    signData.String(),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       "startSign",
			},
			AppConfig: func() _interface.RosenTss {
				localTssData.Party = party
				app := mockedInterface.NewRosenTss(t)
				return app
			},
		},
		{
			name:     "start sign without party",
			expected: "handling incoming sign message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    signData.String(),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       "startSign",
			},
			AppConfig: func() _interface.RosenTss {
				localTssData.Party = nil
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				return app
			},
		},
	}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.AppConfig()
			eddsaSignOpSender := &operationEDDSASign{
				savedData: saveData2,
			}
			eddsaSignOp := &operationEDDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					PeersMap:   make(map[string]string),
					Signatures: make(map[string]string),
					Logger:     logging,
				},
			}
			eddsaSignOp.operationSign.PeersMap[newPartyId.Id] = "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH"

			messageCh := make(chan models.GossipMessage, 100)

			payload := models.Payload{
				Message:    tt.message.Message,
				MessageId:  tt.message.MessageId,
				SenderId:   tt.message.SenderId,
				ReceiverId: tt.message.ReceiverId,
				Name:       tt.message.Name,
			}
			marshal, _ := json.Marshal(payload)
			signature, _ := eddsaSignOpSender.Sign(marshal)
			tt.message.Signature = signature

			messageCh <- tt.message
			go func() {
				time.Sleep(time.Millisecond * 100)
				close(messageCh)
			}()
			errorList := []string{"invalid wire-format", "channel closed"}
			err := eddsaSignOp.Loop(app, messageCh)
			if err != nil && !mockUtils.Contains(err.Error(), errorList) {
				t.Error(err)
			}
		})
	}
}

/*	TestEDDSA_GetClassName
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	-
*/
func TestEDDSA_GetClassName(t *testing.T) {

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "get class name of eddsa sign object",
			expected: "eddsaSign",
		},
	}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				operationSign: sign.OperationSign{
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}

			result := eddsaSignOp.GetClassName()
			if result != tt.expected {
				t.Errorf("GetClassName error = expected %s, got %s", tt.expected, result)
			}

		})
	}
}

func TestEDDSA_signMessage(t *testing.T) {
	// creating fake sign data

	m, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")

	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	tests := []struct {
		name    string
		message []byte
	}{
		{
			name:    "new signature",
			message: m,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				savedData: saveData,
			}

			_, err := eddsaSignOp.Sign(tt.message)
			if err != nil {
				t.Errorf("signMessage error = %v", err)
			}

		})
	}
}

func TestEDDSA_verify(t *testing.T) {
	// creating fake sign data
	saveData, Id1, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	saveData2, Id2, err := mockUtils.LoadEDDSAKeygenFixture(2)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	// creating localTssData and new partyId
	localTssData := models.TssData{
		PartyID: Id1,
	}
	newPartyId := Id2
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID))
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	registerMessage := models.Register{
		Id:        newPartyId.Id,
		Moniker:   newPartyId.Moniker,
		Key:       newPartyId.KeyInt().String(),
		Timestamp: time.Now().Unix() / 60,
		NoAnswer:  false,
	}
	marshal, err := json.Marshal(registerMessage)
	if err != nil {
		t.Errorf("error = %v", err)
	}
	payload := models.Payload{
		Message:    string(marshal),
		MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:   newPartyId.Id,
		ReceiverId: "",
		Name:       "register",
	}
	marshal, err = json.Marshal(payload)
	if err != nil {
		t.Errorf("error = %v", err)
	}

	eddsaSignOp := operationEDDSASign{
		savedData: saveData,
		operationSign: sign.OperationSign{
			LocalTssData: localTssData,
		},
	}
	eddsaSignOpSigner := operationEDDSASign{
		savedData: saveData2,
	}

	tests := []struct {
		name    string
		message []byte
	}{
		{
			name:    "new signature",
			message: marshal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := eddsaSignOpSigner.Sign(tt.message)

			gossipMessage := models.GossipMessage{
				Message:    payload.Message,
				MessageId:  payload.MessageId,
				SenderId:   payload.SenderId,
				ReceiverId: payload.ReceiverId,
				Name:       payload.Name,
				Signature:  signature,
			}
			err = eddsaSignOp.Verify(gossipMessage)
			if err != nil {
				t.Errorf("verify error = %v", err)
			}
		})
	}
}
