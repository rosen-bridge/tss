package ecdsa

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	ecdsaSign "github.com/binance-chain/tss-lib/ecdsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/blake2b"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	mockUtils "rosen-bridge/tss/mocks"
	mockedInterface "rosen-bridge/tss/mocks/app/interface"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"strings"
	"testing"
	"time"
)

/*	TestECDSA_Init
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, receiverId used as test arguments.
	Dependencies:
	- localTssData
	- ecdsaKeygen.LocalPartySaveData
	- storage.LoadECDSAKeygen function
	- network.Publish function
	- rosenTss GetStorage, GetConnection, GetPeerHome, NewMessage functions
*/
func TestECDSA_Init(t *testing.T) {

	// creating fake localTssData
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}

	// reading ecdsaKeygen.LocalPartySaveData data from fixtures
	data, id, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssData
		appConfig    func() _interface.RosenTss
	}{
		{
			name:         "creating partyId message with localTssData, there must be no error",
			receiverId:   "",
			localTssData: localTssData,
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromSign"),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "register",
					})
				return app
			},
		},
		{
			name:         "creating partyId message without localTssData, there must be no error",
			receiverId:   "cahj2pgs4eqvn1eo1tp0",
			localTssData: models.TssData{},
			appConfig: func() _interface.RosenTss {
				storage := mockedStorage.NewStorage(t)
				storage.On("LoadECDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetStorage").Return(storage)
				app.On("GetConnection").Return(conn)
				app.On("GetPeerHome").Return(".rosenTss")
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromSign"),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "register",
					})
				return app
			},
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.appConfig()
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := ecdsaSignOp.Init(app, tt.receiverId)
			if err != nil {
				t.Errorf("Init failed: %v", err)
			}

		})
	}
}

/*	TestECDSA_Loop
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.Message used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- ecdsaKeygen.LocalPartySaveData
	- tss.Party for ecdsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestECDSA_Loop(t *testing.T) {
	// pre-test part, faking data and using mocks

	// reading ecdsaKeygen.LocalPartySaveData from fixtures
	saveData, Id1, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	saveData2, Id2, err := mockUtils.LoadECDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	// creating localTssData and new partyId
	localTssData := models.TssData{
		PartyID: Id1,
	}
	newPartyId := Id2

	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID))
	signDataBytes, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(signDataBytes)

	// creating new tss party for ecdsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	params := tss.NewParameters(
		tss.S256(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))
	party := ecdsaSign.NewLocalParty(signData, params, saveData, outCh, endCh)

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
		StarterId: newPartyId.KeyInt().String(),
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
				MessageId:  "ecdsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       "register",
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					})
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    string(marshal),
						MessageId:  "ecdsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   newPartyId.Id,
						ReceiverId: "",
						Name:       "register",
					})
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
				minutes := time.Now().Unix() / 60
				if minutes%int64(3) == int64(1) {
					app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
						Return(models.GossipMessage{
							Message:    string(setupMessageMarshaled),
							MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
							SenderId:   newPartyId.Id,
							ReceiverId: "",
							Name:       "register",
						})
					conn := mockedNetwork.NewConnection(t)
					conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
					app.On("GetConnection").Return(conn)
				}
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
				localTssData.Params = params
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
				localTssData.Params = params
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
				localTssData.Params = params
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    signData.String(),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   newPartyId.Id,
						ReceiverId: "",
						Name:       "startSign",
					})
				return app
			},
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.AppConfig()
			ecdsaSignOpSender := operationECDSASign{
				savedData: saveData2,
			}
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					PeersMap:   make(map[string]string),
					Signatures: make(map[string]string),
				},
			}
			messageCh := make(chan models.GossipMessage, 100)

			payload := models.Payload{
				Message:    tt.message.Message,
				MessageId:  tt.message.MessageId,
				SenderId:   tt.message.SenderId,
				ReceiverId: tt.message.ReceiverId,
				Name:       tt.message.Name,
			}
			marshal, _ := json.Marshal(payload)
			signature, _ := ecdsaSignOpSender.signMessage(marshal)
			tt.message.Signature = signature

			messageCh <- tt.message
			go func() {
				time.Sleep(time.Second)
				if len(messageCh) == 0 {
					close(messageCh)
				}
			}()
			errorList := []string{"invalid wire-format", "channel closed", "message received"}
			err := ecdsaSignOp.Loop(app, messageCh)
			if err != nil && !mockUtils.Contains(err.Error(), errorList) {
				t.Error(err)
			}
		})
	}
}

/*	TestECDSA_GetClassName
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	-
*/
func TestECDSA_GetClassName(t *testing.T) {

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "get class name of ecdsa sign object",
			expected: "ecdsaSign",
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaSignOp := operationECDSASign{
				operationSign: sign.OperationSign{
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}

			result := ecdsaSignOp.GetClassName()
			if result != tt.expected {
				t.Errorf("GetClassName error = expected %s, got %s", tt.expected, result)
			}

		})
	}
}

/*	TestEDDSA_registerMessageHandler
	TestCases:
	testing message controller, there is 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.GossipMessage, models.SignMessage, models.TssData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- ecdsaKeygen.LocalPartySaveData
	- tss.Party for ecdsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestEDDSA_registerMessageHandler(t *testing.T) {

	// creating new localTssData and new partyIds
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	newPartyId2, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	var localTssData models.TssData
	localTssData.PartyID = newPartyId
	var localTssDataWith2PartyIds models.TssData
	localTssDataWith2PartyIds = localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2))

	// using mocked structures and functions
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetMetaData").Return(
		models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
	)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromSign"),
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "register",
		})

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

	tests := []struct {
		name          string
		gossipMessage models.GossipMessage
		signMessage   models.SignMessage
		localTssData  models.TssData
	}{
		{
			name: "register message with partyId list less than threshold, there must be no error",
			gossipMessage: models.GossipMessage{
				Message:    string(marshal),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "register",
			},
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			localTssData: localTssData,
		},
		{
			name: "register message with partyId list equal to threshold, there must be no error",
			gossipMessage: models.GossipMessage{
				Message:    string(marshal),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "register",
			},
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			localTssData: localTssDataWith2PartyIds,
		},
		{
			name: "register message with partyId list less than threshold, there must be no error, with receiverId",
			gossipMessage: models.GossipMessage{
				Message:    string(marshal),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "cahj2pgs4eqvn1eo1tp0",
				Name:       "register",
			},
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			localTssData: localTssData,
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ecdsaSignOp := operationECDSASign{
				operationSign: sign.OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage:  tt.signMessage,
					PeersMap:     make(map[string]string),
					Signatures:   make(map[string]string),
				},
				savedData: saveData,
			}
			// partyIdMessageHandler
			err = ecdsaSignOp.registerMessageHandler(app, tt.gossipMessage)
			if err != nil {
				t.Error(err)
			}

		})
	}
}

/*	TestECDSA_partyUpdate
	TestCases:
	testing message controller, there is 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.PartyMessage used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- ecdsaKeygen.LocalPartySaveData
	- tss.Party for ecdsaSign.NewLocalParty
*/
func TestECDSA_partyUpdate(t *testing.T) {

	// creating new localTssData and new partyId
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Error(err)
	}

	newParty, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newParty))

	// creating new tss.Party for ecdsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.S256(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan ecdsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	localTssData.Party = ecdsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	for _, party := range localTssData.PartyIds {
		if party.Id == newParty.Id {
			newParty = party
			break
		}
	}

	tests := []struct {
		name    string
		message models.PartyMessage
		wantErr bool
	}{
		{
			name: "PartyUpdate from self to self, there should be an error",
			message: models.PartyMessage{
				To:                      []*tss.PartyID{localTssData.PartyID},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: true,
		},
		{
			name: "PartyUpdate from new party to self there should be no error",
			message: models.PartyMessage{
				To:                      []*tss.PartyID{localTssData.PartyID},
				GetFrom:                 newParty,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: false,
		},
		{
			name: "PartyUpdate from new party to all, there must be no error",
			message: models.PartyMessage{
				To:                      nil,
				GetFrom:                 newParty,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: false,
		},
		{
			name: "PartyUpdate from self to all, there must be no error",
			message: models.PartyMessage{
				To:                      nil,
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: false,
		},
	}

	ecdsaSignOp := operationECDSASign{
		operationSign: sign.OperationSign{
			LocalTssData: localTssData,
			SignMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ecdsaSignOp.partyUpdate(tt.message); (err != nil) != tt.wantErr {
				if !strings.Contains(err.Error(), "invalid wire-format data") {
					t.Errorf("PartyUpdate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

		})
	}
}

/*	TestECDSA_setup
	TestCases:
	testing message controller, there is 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestECDSA_setup(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssDataWith2PartyIds := localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	// using mocked function and struct
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetMetaData").Return(
		models.MetaData{
			PeersCount: 3,
			Threshold:  2,
		})
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "register",
		})

	tests := []struct {
		name         string
		app          _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name:         "creating sign message with partyIds less than threshold, there must be an error",
			app:          app,
			localTssData: localTssData,
			wantErr:      true,
		},
		{
			name:         "creating sign message with partyIds equal to threshold, there must be no error",
			app:          app,
			localTssData: localTssDataWith2PartyIds,
			wantErr:      false,
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := ecdsaSignOp.setup(tt.app)
			if err != nil && !tt.wantErr {
				t.Errorf("Setup error = %v", err)
			}

		})
	}
}

/*	TestECDSA_handleOutMessage
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, tss.Message used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- network.Publish function
	- rosenTss GetConnection, NewMessage functions
*/
func TestECDSA_handleOutMessage(t *testing.T) {
	// creating fake mockUtils.TestUtilsMessage as tss.Message
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating new localTssData
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}

	// using mocked functions and structs
	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "register",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)

	tests := []struct {
		name         string
		app          _interface.RosenTss
		localTssData models.TssData
		tssMessage   tss.Message
	}{
		{
			name:         "creating party message from tss.Message received, there must be no error",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := ecdsaSignOp.handleOutMessage(tt.app, tt.tssMessage)
			if err != nil {
				t.Errorf("handleOutMessage error = %v", err)
			}

		})
	}
}

/*	TestECDSA_handleEndMessage
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, common.SignatureData used as test arguments.
	Dependencies:
	- network.CallBack function
	- rosenTss GetConnection functions
*/
func TestECDSA_handleEndMessage(t *testing.T) {
	// creating fake sign save data
	r, _ := hex.DecodeString("b24c712530dd03739ac87a491e45bd80ea8e3cef19c835bc6ed3262a9794974d")
	s, _ := hex.DecodeString("02fe41c73871ca7ded0ff3e8adc76a64ea93643e75569bd9db8f772166adfc35")
	m, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signature, _ := hex.DecodeString("4d9794972a26d36ebc35c819ef3c8eea80bd451e497ac89a7303dd3025714cb235fcad6621778fdbd99b56753e6493ea646ac7ade8f30fed7dca7138c741fe02")
	saveSign := common.SignatureData{
		R:         r,
		S:         s,
		M:         m,
		Signature: signature,
	}

	app := mockedInterface.NewRosenTss(t)

	conn := mockedNetwork.NewConnection(t)
	conn.On("CallBack", mock.AnythingOfType("string"), mock.AnythingOfType("models.SignData"), mock.AnythingOfType("string")).Return(nil)
	app.On("GetConnection").Return(conn)

	tests := []struct {
		name          string
		app           _interface.RosenTss
		signatureData *common.SignatureData
	}{
		{
			name:          "handling sign data in the end of loop, there must be no error from callback",
			app:           app,
			signatureData: &saveSign,
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaSignOp := operationECDSASign{
				operationSign: sign.OperationSign{},
			}
			err := ecdsaSignOp.handleEndMessage(tt.app, tt.signatureData)
			if err != nil {
				t.Errorf("handleOutMessage error = %v", err)
			}

		})
	}
}

/*	TestECDSA_handleOutMessage
	TestCases:
	testing message controller, there are 2 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, tss.Message, common.SignatureData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- network Publish, CallBack functions
	- rosenTss GetConnection, NewMessage functions
*/
func TestECDSA_gossipMessageHandler(t *testing.T) {
	// creating fake sign data
	r, _ := hex.DecodeString("b24c712530dd03739ac87a491e45bd80ea8e3cef19c835bc6ed3262a9794974d")
	s, _ := hex.DecodeString("02fe41c73871ca7ded0ff3e8adc76a64ea93643e75569bd9db8f772166adfc35")
	m, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signature, _ := hex.DecodeString("4d9794972a26d36ebc35c819ef3c8eea80bd451e497ac89a7303dd3025714cb235fcad6621778fdbd99b56753e6493ea646ac7ade8f30fed7dca7138c741fe02")
	saveSign := common.SignatureData{
		R:         r,
		S:         s,
		M:         m,
		Signature: signature,
	}
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "register",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	conn.On("CallBack", mock.AnythingOfType("string"), mock.AnythingOfType("models.SignData"), mock.AnythingOfType("string")).Return(
		fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)

	tests := []struct {
		name          string
		expected      string
		app           _interface.RosenTss
		signatureData *common.SignatureData
		localTssData  models.TssData
		tssMessage    tss.Message
	}{
		{
			name:          "handling sign",
			expected:      "there should be an \"message received\" error",
			app:           app,
			signatureData: &saveSign,
			localTssData:  localTssData,
		},
		{
			name:         "party message",
			expected:     "there should be an \"message received\" error",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}

	logging, _ = mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			outCh := make(chan tss.Message, len(ecdsaSignOp.operationSign.LocalTssData.PartyIds))
			endCh := make(chan common.SignatureData, len(ecdsaSignOp.operationSign.LocalTssData.PartyIds))
			switch tt.name {

			case "handling sign":
				endCh <- *tt.signatureData
			case "party message":
				outCh <- tt.tssMessage
			}
			result, err := ecdsaSignOp.gossipMessageHandler(tt.app, outCh, endCh)
			if err != nil {
				assert.Equal(t, result, false)
				if err.Error() != "message received" {
					t.Errorf("gossipMessageHandler error = %v", err)
				}
			} else {
				assert.Equal(t, result, true)
			}

		})
	}
}

func TestECDSA_signMessage(t *testing.T) {
	// creating fake sign data

	m, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")

	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
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
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
			}

			_, err := ecdsaSignOp.signMessage(tt.message)
			if err != nil {
				t.Errorf("signMessage error = %v", err)
			}

		})
	}
}

func TestECDSA_verify(t *testing.T) {
	// creating fake sign data
	saveData, Id1, err := mockUtils.LoadECDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	saveData2, Id2, err := mockUtils.LoadECDSAKeygenFixture(2)
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

	ecdsaSignOp := operationECDSASign{
		savedData: saveData,
		operationSign: sign.OperationSign{
			LocalTssData: localTssData,
		},
	}
	ecdsaSignOpSigner := operationECDSASign{
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
			signature, err := ecdsaSignOpSigner.signMessage(tt.message)

			gossipMessage := models.GossipMessage{
				Message:    payload.Message,
				MessageId:  payload.MessageId,
				SenderId:   payload.SenderId,
				ReceiverId: payload.ReceiverId,
				Name:       payload.Name,
				Signature:  signature,
			}
			err = ecdsaSignOp.verify(gossipMessage)
			if err != nil {
				t.Errorf("verify error = %v", err)
			}
		})
	}
}

func TestECDSA_newMessage(t *testing.T) {
	// creating localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	message := models.GossipMessage{
		Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:   "cahj2pgs4eqvn1eo1tp0",
		ReceiverId: "",
		Name:       "register",
	}
	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(message)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)

	tests := []struct {
		name string
	}{
		{
			name: "create and send new message to p2p",
		},
	}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaSignOp := operationECDSASign{
				savedData: saveData,
				operationSign: sign.OperationSign{
					LocalTssData: localTssData,
				},
			}

			err := ecdsaSignOp.newMessage(app, message.ReceiverId, message.SenderId, message.Message, message.MessageId, message.Name)
			if err != nil && err.Error() != "message received" {
				t.Errorf("newMessage error = %v", err)
			}
		})
	}
}
