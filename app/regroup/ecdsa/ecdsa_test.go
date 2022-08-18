package ecdsa

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	ecdsaRegroup "github.com/binance-chain/tss-lib/ecdsa/resharing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/regroup"
	mockUtils "rosen-bridge/tss/mocks"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"testing"
	"time"

	mockedInterface "rosen-bridge/tss/mocks/app/interface"
)

/*	TestECDSA_Init
	TestCases:
	testing message controller, there are 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, receiverId used as test arguments.
	Dependencies:
	- localTssData
	- ecdsaKeygen.LocalPartySaveData
	- storage.LoadECDSAKeygen function
	- network.Publish function
	- rosenTss GetStorage, GetConnection, GetPrivate, SetPrivate, NewMessage functions
*/
func TestECDSA_Init(t *testing.T) {
	// creating fake localTssData
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}

	// reading ecdsaKeygen.LocalPartySaveData data from fixtures
	data, id, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	// using mock functions

	priv, _, _, err := utils.GenerateECDSAKey()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssRegroupData
		appConfig    func() _interface.RosenTss
	}{
		{
			name: "creating partyId from Regroup message state 0",
			localTssData: models.TssRegroupData{
				PeerState: 0,
			},
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				store := mockedStorage.NewStorage(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("GetPeerHome").Return("/tmp/.rosenTss")
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})

				store.On("LoadECDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				app.On("GetStorage").Return(store)
				return app
			},
		},
		{
			name: "creating partyId from Regroup message state 1 without key",
			localTssData: models.TssRegroupData{
				PeerState: 1,
			},
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})

				app.On("GetPrivate", mock.AnythingOfType("string")).Return("", nil)
				app.On("SetPrivate", mock.AnythingOfType("models.Private")).Return(nil)
				return app
			},
		},
		{
			name: "creating partyId from Regroup message state 1 with key",
			localTssData: models.TssRegroupData{
				PeerState: 1,
			},
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				app.On("GetPrivate", mock.AnythingOfType("string")).Return(hex.EncodeToString(priv), nil)
				return app
			},
		},
	}
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.appConfig()
			ecdsaRegroupOp := operationECDSARegroup{
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: tt.localTssData,
				},
			}

			err = ecdsaRegroupOp.Init(app, "")
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
	- tss.Party for ecdsaKeygen.NewLocalParty
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestECDSA_Loop(t *testing.T) {
	// pre-test part, faking data and using mocks

	// reading ecdsaKeygen.LocalPartySaveData from fixtures
	// creating new localTssData and new partyId
	saveData, oldPartyId, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}
	partyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData := models.TssRegroupData{
		PeerState: 1,
		PartyID:   oldPartyId,
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), partyId))
	localTssData.OldPartyIds = tss.SortPartyIDs(
		append(localTssData.OldPartyIds.ToUnSorted(), partyId))
	newParty, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newParty))

	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), oldPartyId))

	// creating new tss party for ecdsa sign
	newCtx := tss.NewPeerContext(localTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(localTssData.OldPartyIds)
	localTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.S256(), oldCtx, newCtx, localTssData.PartyID, 3, 1,
		len(localTssData.NewPartyIds), 2)

	outCh := make(chan tss.Message, len(localTssData.NewPartyIds))
	endCh := make(chan ecdsaKeygen.LocalPartySaveData, len(localTssData.NewPartyIds))
	party := ecdsaRegroup.NewLocalParty(localTssData.RegroupingParams, saveData, outCh, endCh)

	partyIDMessage := fmt.Sprintf("%s,%s,%d,%s,%d", partyId.Id, partyId.Moniker, partyId.KeyInt(), "fromRegroup", 0)

	partyMessage := models.PartyMessage{
		Message:     []byte(partyIDMessage),
		IsBroadcast: true,
		GetFrom:     partyId,
		To:          []*tss.PartyID{localTssData.PartyID},
	}

	partyMessageBytes, err := json.Marshal(partyMessage)
	if err != nil {
		t.Error("failed to marshal message", err)
	}

	// creating new app from mocked rosenTss, handling mock function done in each case separately
	app := mockedInterface.NewRosenTss(t)

	// test cases
	tests := []struct {
		name      string
		expected  string
		message   models.GossipMessage
		AppConfig func()
	}{
		{
			name:     "partyId",
			expected: "handling incoming partyId message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    partyIDMessage,
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			AppConfig: func() {},
		},
		{
			name:     "partyMsg",
			expected: "handling incoming partyMsg message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    hex.EncodeToString(partyMessageBytes),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyMsg",
			},
			AppConfig: func() {
				localTssData.Party = party
			},
		},
		{
			name:     "regroup with party",
			expected: "handling incoming regroup message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    "regroup pre params creates.",
				MessageId:  "regroup",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "regroup",
			},
			AppConfig: func() {
				localTssData.Party = party
			},
		},
		{
			name:     "regroup without party",
			expected: "handling incoming regroup message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    "regroup pre params creates.",
				MessageId:  "regroup",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "regroup",
			},
			AppConfig: func() {
				localTssData.Party = nil
			},
		},
	}

	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.AppConfig()

			ecdsaRegroupOp := operationECDSARegroup{
				savedData: saveData,
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: localTssData,
					RegroupMessage: models.RegroupMessage{
						PeerState:    0,
						NewThreshold: 3,
						OldThreshold: 2,
						PeersCount:   4,
						Crypto:       "ecdsa",
					},
				},
			}

			messageCh := make(chan models.GossipMessage, 100)

			messageCh <- tt.message
			go func() {
				time.Sleep(time.Millisecond * 100)
				close(messageCh)
			}()
			errorList := []string{"invalid wire-format", "channel closed", "threshold", "len(ks)"}
			err := ecdsaRegroupOp.Loop(app, messageCh)
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
			name:     "get class name of ecdsa regroup object",
			expected: "ecdsaRegroup",
		},
	}
	logging, _ = mockUtils.InitLog("ecdsa-regroup")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaRegroupOp := operationECDSARegroup{}

			result := ecdsaRegroupOp.GetClassName()
			if result != tt.expected {
				t.Errorf("GetClassName error = expected %s, got %s", tt.expected, result)
			}

		})
	}
}

/*	TestECDSA_partyIdMessageHandler
	TestCases:
	testing message controller, there is 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.GossipMessage, models.TssData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestECDSA_partyIdMessageHandler(t *testing.T) {

	data, id, _ := mockUtils.LoadECDSAKeygenFixture(0)
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("createNewParty error = %v", err)
	}
	tests := []struct {
		name          string
		peerState     int
		gossipMessage models.GossipMessage
		wantErr       bool
		appConfig     func() _interface.RosenTss
		dataConfig    func() models.TssRegroupData
	}{
		{
			name:      "handling new party with state 0 with true threshold",
			peerState: 0,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 0),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{}
				id1, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				id2, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				localTssData.NewPartyIds = tss.SortPartyIDs(
					append(localTssData.NewPartyIds.ToUnSorted(), id1))
				localTssData.NewPartyIds = tss.SortPartyIDs(
					append(localTssData.NewPartyIds.ToUnSorted(), id2))
				localTssData.OldPartyIds = tss.SortPartyIDs(
					append(localTssData.OldPartyIds.ToUnSorted(), id))
				return localTssData
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 0 with false threshold",
			peerState: 0,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 0),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{}
				id1, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				localTssData.NewPartyIds = tss.SortPartyIDs(
					append(localTssData.NewPartyIds.ToUnSorted(), id1))
				return localTssData
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 1 with true threshold",
			peerState: 1,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 1),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", id.Id, id.Moniker, id.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{}
				id1, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				id2, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				localTssData.NewPartyIds = tss.SortPartyIDs(
					append(localTssData.NewPartyIds.ToUnSorted(), id))
				localTssData.OldPartyIds = tss.SortPartyIDs(
					append(localTssData.OldPartyIds.ToUnSorted(), id1))
				localTssData.OldPartyIds = tss.SortPartyIDs(
					append(localTssData.OldPartyIds.ToUnSorted(), id2))
				return localTssData
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 1 with false threshold",
			peerState: 1,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 1),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", id.Id, id.Moniker, id.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{}
				id1, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				localTssData.OldPartyIds = tss.SortPartyIDs(
					append(localTssData.OldPartyIds.ToUnSorted(), id1))

				return localTssData
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 0 with wrong message",
			peerState: 0,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromKeygen", 0),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{}
				return localTssData
			},
			wantErr: true,
		},
		{
			name:      "handling new party with state 0 with receiver not equal to local",
			peerState: 0,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromKeygen", 0),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "cahj2pgs4eqvn1eo1tp1",
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{
					PartyID: id,
				}
				return localTssData
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 0 party list less than peersCount",
			peerState: 0,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 0),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: newPartyId.Id,
				Name:       "partyId",
			},
			appConfig: func() _interface.RosenTss {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s,%d", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromRegroup", 0),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: newPartyId.Id,
						Name:       "partyId",
					})
				return app
			},
			dataConfig: func() models.TssRegroupData {
				localTssData := models.TssRegroupData{}
				id1, err := mockUtils.CreateNewECDSAPartyId()
				if err != nil {
					t.Errorf("createNewParty error = %v", err)
				}
				localTssData.NewPartyIds = tss.SortPartyIDs(
					append(localTssData.NewPartyIds.ToUnSorted(), id))
				localTssData.NewPartyIds = tss.SortPartyIDs(
					append(localTssData.NewPartyIds.ToUnSorted(), id1))
				return localTssData
			},
			wantErr: false,
		},
	}
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.appConfig()
			localTssData := tt.dataConfig()

			ecdsaRegroupOp := operationECDSARegroup{
				savedData: data,
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PeerState:   tt.peerState,
						PartyID:     newPartyId,
						OldPartyIds: localTssData.OldPartyIds,
						NewPartyIds: localTssData.NewPartyIds,
					},
					RegroupMessage: models.RegroupMessage{
						PeerState:    tt.peerState,
						NewThreshold: 1,
						OldThreshold: 1,
						PeersCount:   5,
						Crypto:       "ecdsa",
					},
				},
			}

			err = ecdsaRegroupOp.partyIdMessageHandler(app, tt.gossipMessage)
			if err != nil && !tt.wantErr {
				t.Errorf("partyIdMessageHandler has error: %v", err)
			}
		})
	}
}

/*	TestECDSA_partyUpdate
	TestCases:
	testing message controller, there is 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.PartyMessage used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- tss.Party for ecdsaKeygen.NewLocalParty
*/
func TestECDSA_partyUpdate(t *testing.T) {
	// creating new localTssData and new partyId
	saveData, oldPartyId, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}
	partyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData := models.TssRegroupData{
		PeerState: 1,
		PartyID:   oldPartyId,
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), partyId))
	localTssData.OldPartyIds = tss.SortPartyIDs(
		append(localTssData.OldPartyIds.ToUnSorted(), partyId))
	newParty, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newParty))

	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), oldPartyId))

	// creating new tss.Party for ecdsa sign
	newCtx := tss.NewPeerContext(localTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(localTssData.OldPartyIds)
	localTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.Edwards(), oldCtx, newCtx, localTssData.PartyID, 3, 1,
		len(localTssData.NewPartyIds), 2)
	partiesLength := len(localTssData.NewPartyIds) + len(localTssData.OldPartyIds)

	outCh := make(chan tss.Message, partiesLength)
	endCh := make(chan ecdsaKeygen.LocalPartySaveData, partiesLength)
	localTssData.Party = ecdsaRegroup.NewLocalParty(localTssData.RegroupingParams, saveData, outCh, endCh)

	tests := []struct {
		name         string
		localTssData models.TssRegroupData
		message      models.PartyMessage
		wantErr      bool
	}{
		{
			name: "PartyUpdate to old Committee, there should be an error",
			localTssData: models.TssRegroupData{
				PartyID:     oldPartyId,
				NewPartyIds: localTssData.NewPartyIds,
				OldPartyIds: localTssData.OldPartyIds,
				PeerState:   0,
				Party:       localTssData.Party,
			},
			message: models.PartyMessage{
				To:                      []*tss.PartyID{oldPartyId},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
		},
		{
			name: "PartyUpdate to new Committee, there should be an error",
			localTssData: models.TssRegroupData{
				PartyID:     newParty,
				NewPartyIds: localTssData.NewPartyIds,
				OldPartyIds: localTssData.OldPartyIds,
				PeerState:   1,
				Party:       localTssData.Party,
			},
			message: models.PartyMessage{
				To:                      []*tss.PartyID{newParty},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        false,
				IsToOldAndNewCommittees: false,
			},
		},
		{
			name: "PartyUpdate to nil destination, there should be an error",
			localTssData: models.TssRegroupData{
				PartyID:     newParty,
				NewPartyIds: localTssData.NewPartyIds,
				OldPartyIds: localTssData.OldPartyIds,
				PeerState:   1,
				Party:       localTssData.Party,
			},
			message: models.PartyMessage{
				To:                      nil,
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        false,
				IsToOldAndNewCommittees: false,
			},
		},
	}
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaRegroupOp := operationECDSARegroup{
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: tt.localTssData,
					RegroupMessage: models.RegroupMessage{
						PeerState:    tt.localTssData.PeerState,
						NewThreshold: 3,
						OldThreshold: 2,
						PeersCount:   4,
						Crypto:       "ecdsa",
					},
				},
			}
			errorList := []string{"invalid wire-format", "nil destination during regrouping"}

			if err := ecdsaRegroupOp.partyUpdate(tt.message); err != nil && !mockUtils.Contains(err.Error(), errorList) {
				t.Errorf("partyUpdate has error: %v", err)
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
	_, oldPartyId, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}
	partyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData := models.TssRegroupData{
		PeerState: 1,
		PartyID:   oldPartyId,
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), partyId))
	localTssData.OldPartyIds = tss.SortPartyIDs(
		append(localTssData.OldPartyIds.ToUnSorted(), partyId))
	newParty, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newParty))

	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)
	tests := []struct {
		name      string
		sendCh    chan []byte
		peerState int
	}{
		{
			name:      "creating Regroup message with state 0",
			peerState: 0,
		},
		{
			name:      "creating Regroup message with state 1",
			peerState: 1,
		},
	}
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaRegroupOp := operationECDSARegroup{
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PeerState:   tt.peerState,
						OldPartyIds: localTssData.OldPartyIds,
						NewPartyIds: localTssData.NewPartyIds,
						PartyID:     localTssData.PartyID,
					},
					RegroupMessage: models.RegroupMessage{
						PeerState:    tt.peerState,
						NewThreshold: 3,
						OldThreshold: 2,
						PeersCount:   4,
						Crypto:       "ecdsa",
					},
				},
			}
			err := ecdsaRegroupOp.setup(app)
			if err != nil {
				t.Errorf("setup error = %v", err)
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
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}

	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	store := mockedStorage.NewStorage(t)
	store.On("WriteData",
		mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	app.On("GetConnection").Return(conn)
	app.On("GetStorage").Return(store)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")

	tests := []struct {
		name         string
		app          _interface.RosenTss
		localTssData models.TssData
		tssMessage   tss.Message
	}{
		{
			name:         "creating party message",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaRegroupOp := operationECDSARegroup{
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PartyID: tt.localTssData.PartyID,
					},
				},
			}
			err := ecdsaRegroupOp.handleOutMessage(tt.app, tt.tssMessage)
			if err != nil {
				t.Errorf("handleOutMessage error = %v", err)
			}

		})
	}
}

/*	TestECDSA_handleEndMessage
	TestCases:
	testing message controller, there are 2 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is ecdsaKeygen.LocalPartySaveData used as test arguments.
	Dependencies:
	- storage.WriteData function
	- rosenTss GetStorage, GetPeerHome functions
*/
func TestECDSA_handleEndMessage(t *testing.T) {
	// loading fixture data
	fixture, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name        string
		regroupData ecdsaKeygen.LocalPartySaveData
		peerState   int
	}{
		{
			name:        "creating ecdsa keygen data",
			regroupData: fixture,
			peerState:   1,
		},
		{
			name:        "creating ecdsa keygen data",
			regroupData: fixture,
			peerState:   0,
		},
	}

	// using mocked structs and functions
	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	conn := mockedNetwork.NewConnection(t)
	conn.On("CallBack", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetStorage").Return(store)
	app.On("GetConnection").Return(conn)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaRegroupOp := operationECDSARegroup{
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PeerState: tt.peerState,
					},
				},
			}
			err := ecdsaRegroupOp.handleEndMessage(app, tt.regroupData)
			if err != nil {
				t.Errorf("handleEndMessage failed: %v", err)
			}
		})
	}
}

/*	TestECDSA_handleOutMessage
	TestCases:
	testing message controller, there are 2 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, tss.Message, ecdsaKeygen.LocalPartySaveData used as test arguments.
	Dependencies:
	- ecdsaKeygen.LocalPartySaveData
	- localTssData models.TssData
	- network Publish, CallBack functions
	- storage WriteData function
	- rosenTss GetConnection, NewMessage, GetPeerHome, GetStorage  functions
*/
func TestECDSA_gossipMessageHandler(t *testing.T) {
	// reding fixutre data
	fixture, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}

	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating localTssDatas and new partyIds
	partyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssData := models.TssRegroupData{
		PartyID: partyId,
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newPartyId))
	localTssData.OldPartyIds = tss.SortPartyIDs(
		append(localTssData.OldPartyIds.ToUnSorted(), partyId))
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), partyId))
	localTssData.PeerState = 1
	// using mocked structs and functions

	tests := []struct {
		name         string
		regroupData  ecdsaKeygen.LocalPartySaveData
		localTssData models.TssRegroupData
		tssMessage   tss.Message
		appConfig    func() _interface.RosenTss
	}{
		{
			name:         "save regroup",
			regroupData:  fixture,
			localTssData: localTssData,
			appConfig: func() _interface.RosenTss {
				store := mockedStorage.NewStorage(t)
				app := mockedInterface.NewRosenTss(t)
				store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("message received"))
				app.On("GetPeerHome").Return("/tmp/.rosenTss")
				app.On("GetStorage").Return(store)
				return app
			},
		},
		{
			name:         "party message",
			localTssData: localTssData,
			tssMessage:   &message,
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
				app.On("GetConnection").Return(conn)
				return app
			},
		},
	}
	logging, err = mockUtils.InitLog("ecdsa-regroup")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.appConfig()
			ecdsaRegroupOp := operationECDSARegroup{
				operationRegroup: regroup.OperationRegroup{
					LocalTssData: tt.localTssData,
				},
			}
			partiesLength := len(localTssData.NewPartyIds) + len(localTssData.OldPartyIds)

			outCh := make(chan tss.Message, partiesLength)
			endCh := make(chan ecdsaKeygen.LocalPartySaveData, partiesLength)
			switch tt.name {
			case "save regroup":
				endCh <- tt.regroupData
			case "party message":
				outCh <- tt.tssMessage
			}
			result, err := ecdsaRegroupOp.gossipMessageHandler(app, outCh, endCh)
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
