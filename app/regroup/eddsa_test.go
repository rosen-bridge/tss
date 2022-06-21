package regroup

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaRegroup "github.com/binance-chain/tss-lib/eddsa/resharing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	mockUtils "rosen-bridge/tss/mocks"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"testing"
	"time"

	mockedInterface "rosen-bridge/tss/mocks/app/interface"
)

/*	TestEDDSA_Init
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, receiverId used as test arguments.
	Dependencies:
	- localTssData
	- eddsaKeygen.LocalPartySaveData
	- storage.LoadEDDSAKeygen function
	- network.Publish function
	- rosenTss GetStorage, GetConnection, GetPrivate, Setprivate, NewMessage functions
*/
func TestEDDSA_Init(t *testing.T) {
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
	app := mockedInterface.NewRosenTss(t)
	store := mockedStorage.NewStorage(t)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)
	app.On("GetPeerHome").Return(".rosenTss")
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    fmt.Sprintf("%s,%s,%d,%s,%d", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromRegroup", 0),
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})

	priv, _, _, err := utils.GenerateEDDSAKey()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssRegroupData
		appConfig    func()
	}{
		{
			name: "creating partyId from Regroup message state 0",
			localTssData: models.TssRegroupData{
				PeerState: 0,
			},
			appConfig: func() {
				store.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				app.On("GetStorage").Return(store)
			},
		},
		{
			name: "creating partyId from Regroup message state 1 without key",
			localTssData: models.TssRegroupData{
				PeerState: 1,
			},
			appConfig: func() {
				app.On("GetPrivate", mock.AnythingOfType("string")).Return("", nil)
				app.On("SetPrivate", mock.AnythingOfType("models.Private")).Return(nil)
			},
		},
		{
			name: "creating partyId from Regroup message state 1 with key",
			localTssData: models.TssRegroupData{
				PeerState: 1,
			},
			appConfig: func() {
				app.On("GetPrivate", mock.AnythingOfType("string")).Return(hex.EncodeToString(priv), nil)
			},
		},
		{
			name: "creating partyId from Regroup message state 2",
			localTssData: models.TssRegroupData{
				PeerState: 2,
			},
			appConfig: func() {
				store.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
				app.On("GetStorage").Return(store)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.appConfig()
			eddsaRegroupOp := operationEDDSARegroup{
				OperationRegroup: OperationRegroup{
					LocalTssData: tt.localTssData,
				},
			}

			err = eddsaRegroupOp.Init(app, "")
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
	- tss.Party for eddsaKeygen.NewLocalParty
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestEDDSA_Loop(t *testing.T) {
	// pre-test part, faking data and using mocks

	// reading eddsaKeygen.LocalPartySaveData from fixtures
	saveData, Id1, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	_, Id4, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	Id2, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)

	}
	Id3, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)

	}
	// creating localTssData and new partyId
	localTssData := models.TssRegroupData{
		PartyID: Id1,
	}

	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), Id2))
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), Id3))
	localTssData.OldPartyIds = tss.SortPartyIDs(
		append(localTssData.OldPartyIds.ToUnSorted(), localTssData.PartyID))
	localTssData.OldPartyIds = tss.SortPartyIDs(
		append(localTssData.OldPartyIds.ToUnSorted(), Id4))

	// creating new tss party for eddsa sign
	newCtx := tss.NewPeerContext(localTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(localTssData.OldPartyIds)
	localTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.S256(), oldCtx, newCtx, localTssData.PartyID, 3, 1,
		len(localTssData.NewPartyIds), 2)

	outCh := make(chan tss.Message, len(localTssData.NewPartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.NewPartyIds))
	party := eddsaRegroup.NewLocalParty(localTssData.RegroupingParams, saveData, outCh, endCh)

	partyIDMessage := fmt.Sprintf("%s,%s,%d,%s,%d", Id2.Id, Id2.Moniker, Id2.KeyInt(), "fromRegroup", 0)

	partyMessage := models.PartyMessage{
		Message:     []byte(partyIDMessage),
		IsBroadcast: true,
		GetFrom:     Id2,
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
		message   models.Message
		AppConfig func()
	}{
		{
			name:     "partyId",
			expected: "handling incoming partyId message from p2p, there must be no error out of err list",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    partyIDMessage,
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyId",
				},
			},
			AppConfig: func() {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    "regroup pre params creates.",
						MessageId:  "regroup",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "regroup",
					})

			},
		},
		{
			name:     "partyMsg",
			expected: "handling incoming partyMsg message from p2p, there must be no error out of err list",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    hex.EncodeToString(partyMessageBytes),
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyMsg",
				},
			},
			AppConfig: func() {
				localTssData.Party = party
			},
		},
		{
			name:     "regroup with party",
			expected: "handling incoming regroup message from p2p, there must be no error out of err list",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "regroup pre params creates.",
					MessageId:  "regroup",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "regroup",
				},
			},
			AppConfig: func() {
				localTssData.Party = party
			},
		},
		{
			name:     "regroup without party",
			expected: "handling incoming regroup message from p2p, there must be no error out of err list",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "regroup pre params creates.",
					MessageId:  "regroup",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "regroup",
				},
			},
			AppConfig: func() {
				localTssData.Party = nil
				conn := mockedNetwork.NewConnection(t)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    "regroup pre params creates.",
						MessageId:  "regroup",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "regroup",
					})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.AppConfig()
			eddsaRegroupOp := operationEDDSARegroup{
				savedData: saveData,
				OperationRegroup: OperationRegroup{
					LocalTssData: localTssData,
					RegroupMessage: models.RegroupMessage{
						PeerState:    0,
						NewThreshold: 3,
						OldThreshold: 2,
						PeersCount:   4,
						Crypto:       "eddsa",
					},
				},
			}

			messageCh := make(chan models.Message, 100)

			messageCh <- tt.message
			go func() {
				time.Sleep(time.Millisecond * 100)
				close(messageCh)
			}()
			errorList := []string{"invalid wire-format", "channel closed", "threshold", "len(ks)"}
			err := eddsaRegroupOp.Loop(app, messageCh)
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
			name:     "get class name of eddsa regroup object",
			expected: "eddsaRegroup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaRegroupOp := operationEDDSARegroup{}

			result := eddsaRegroupOp.GetClassName()
			if result != tt.expected {
				t.Errorf("GetClassName error = expected %s, got %s", tt.expected, result)
			}

		})
	}
}

/*	TestEDDSA_partyIdMessageHandler
	TestCases:
	testing message controller, there is 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.GossipMessage, models.TssData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestEDDSA_partyIdMessageHandler(t *testing.T) {}

/*	TestEDDSA_partyUpdate
	TestCases:
	testing message controller, there is 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.PartyMessage used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- tss.Party for eddsaKeygen.NewLocalParty
*/
func TestEDDSA_partyUpdate(t *testing.T) {}

/*	TestEDDSA_setup
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
func TestEDDSA_setup(t *testing.T) {}

/*	TestEDDSA_handleOutMessage
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
func TestEDDSA_handleOutMessage(t *testing.T) {}

/*	TestEDDSA_handleEndMessage
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is eddsaKeygen.LocalPartySaveData used as test arguments.
	Dependencies:
	- storage.WriteData function
	- rosenTss GetStorage, GetPeerHome functions
*/
func TestEDDSA_handleEndMessage(t *testing.T) {}

/*	TestEDDSA_handleOutMessage
	TestCases:
	testing message controller, there are 2 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, tss.Message, eddsaKeygen.LocalPartySaveData used as test arguments.
	Dependencies:
	- eddsaKeygen.LocalPartySaveData
	- localTssData models.TssData
	- network Publish, CallBack functions
	- storage WriteData function
	- rosenTss GetConnection, NewMessage, GetPeerHome, GetStorage  functions
*/
func TestEDDSA_gossipMessageHandler(t *testing.T) {}
