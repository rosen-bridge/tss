package regroup

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaRegroup "github.com/binance-chain/tss-lib/eddsa/resharing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	_interface "rosen-bridge/tss/app/interface"
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
	// creating new localTssData and new partyId
	saveData, oldPartyId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}
	partyId, err := mockUtils.CreateNewEDDSAPartyId()
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
	newParty, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newParty))

	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), oldPartyId))

	// creating new tss party for eddsa sign
	newCtx := tss.NewPeerContext(localTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(localTssData.OldPartyIds)
	localTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.S256(), oldCtx, newCtx, localTssData.PartyID, 3, 1,
		len(localTssData.NewPartyIds), 2)

	outCh := make(chan tss.Message, len(localTssData.NewPartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.NewPartyIds))
	party := eddsaRegroup.NewLocalParty(localTssData.RegroupingParams, saveData, outCh, endCh)

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
func TestEDDSA_partyIdMessageHandler(t *testing.T) {

	data, id, _ := mockUtils.LoadEDDSAKeygenFixture(0)
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
	//priv, _, _, err := utils.GenerateEDDSAKey()
	//if err != nil {
	//	t.Fatal(err)
	//}

	tests := []struct {
		name          string
		peerState     int
		gossipMessage models.GossipMessage
		wantErr       bool
	}{
		{
			name:      "handling new party with state 0",
			peerState: 0,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", id.Id, id.Moniker, id.KeyInt(), "fromRegroup", 0),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 1",
			peerState: 1,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", id.Id, id.Moniker, id.KeyInt(), "fromRegroup", 1),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 2",
			peerState: 2,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", id.Id, id.Moniker, id.KeyInt(), "fromRegroup", 2),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			wantErr: false,
		},
		{
			name:      "handling new party with state 2 with wrong message",
			peerState: 2,
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s,%d", id.Id, id.Moniker, id.KeyInt(), "fromKeygen", 2),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
			if err != nil {
				t.Errorf("createNewParty error = %v", err)
			}
			eddsaRegroupOp := operationEDDSARegroup{
				savedData: data,
				OperationRegroup: OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PeerState: tt.peerState,
						PartyID:   newPartyId,
					},
					RegroupMessage: models.RegroupMessage{
						PeerState:    tt.peerState,
						NewThreshold: 3,
						OldThreshold: 2,
						PeersCount:   4,
						Crypto:       "eddsa",
					},
				},
			}

			err = eddsaRegroupOp.partyIdMessageHandler(app, tt.gossipMessage)
			if err != nil && !tt.wantErr {
				t.Errorf("partyIdMessageHandler has error: %v", err)
			}
		})
	}
}

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
func TestEDDSA_partyUpdate(t *testing.T) {
	// creating new localTssData and new partyId
	saveData, oldPartyId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}
	partyId, err := mockUtils.CreateNewEDDSAPartyId()
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
	newParty, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newParty))

	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), oldPartyId))

	// creating new tss.Party for eddsa sign
	newCtx := tss.NewPeerContext(localTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(localTssData.OldPartyIds)
	localTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.Edwards(), oldCtx, newCtx, localTssData.PartyID, 3, 1,
		len(localTssData.NewPartyIds), 2)
	partiesLength := len(localTssData.NewPartyIds) + len(localTssData.OldPartyIds)

	outCh := make(chan tss.Message, partiesLength)
	endCh := make(chan eddsaKeygen.LocalPartySaveData, partiesLength)
	localTssData.Party = eddsaRegroup.NewLocalParty(localTssData.RegroupingParams, saveData, outCh, endCh)

	tests := []struct {
		name         string
		localTssData models.TssRegroupData
		message      models.PartyMessage
		wantErr      bool
	}{
		{
			name: "PartyUpdate to old and new Committee, there should be an error",
			localTssData: models.TssRegroupData{
				PartyID:     partyId,
				NewPartyIds: localTssData.NewPartyIds,
				OldPartyIds: localTssData.OldPartyIds,
				Party:       localTssData.Party,
				PeerState:   2,
			},
			message: models.PartyMessage{
				To:                      []*tss.PartyID{partyId},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: true,
			},
		},
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
				To:                      []*tss.PartyID{newParty},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        false,
				IsToOldAndNewCommittees: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaRegroupOp := operationEDDSARegroup{
				OperationRegroup: OperationRegroup{
					LocalTssData: tt.localTssData,
					RegroupMessage: models.RegroupMessage{
						PeerState:    tt.localTssData.PeerState,
						NewThreshold: 3,
						OldThreshold: 2,
						PeersCount:   4,
						Crypto:       "eddsa",
					},
				},
			}
			errorList := []string{"invalid wire-format", "nil destination during regrouping"}

			if err := eddsaRegroupOp.partyUpdate(tt.message); err != nil && !mockUtils.Contains(err.Error(), errorList) {
				t.Errorf("partyUpdate has error: %v", err)
			}

		})
	}
}

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
func TestEDDSA_setup(t *testing.T) {
	// creating new localTssData and new partyId
	_, oldPartyId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}
	partyId, err := mockUtils.CreateNewEDDSAPartyId()
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
	newParty, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), newParty))

	localTssData.NewPartyIds = tss.SortPartyIDs(
		append(localTssData.NewPartyIds.ToUnSorted(), oldPartyId))

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
		{
			name:      "creating Regroup message with state 2",
			peerState: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaRegroupOp := operationEDDSARegroup{
				OperationRegroup: OperationRegroup{
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
						Crypto:       "eddsa",
					},
				},
			}
			err := eddsaRegroupOp.setup(app)
			if err != nil {
				t.Errorf("setup error = %v", err)
			}
		})
	}
}

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
func TestEDDSA_handleOutMessage(t *testing.T) {
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaRegroupOp := operationEDDSARegroup{
				OperationRegroup: OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PartyID: tt.localTssData.PartyID,
					},
				},
			}
			err := eddsaRegroupOp.handleOutMessage(tt.app, tt.tssMessage)
			if err != nil {
				t.Errorf("handleOutMessage error = %v", err)
			}

		})
	}
}

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
func TestEDDSA_handleEndMessage(t *testing.T) {
	// loading fixture data
	fixture, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name        string
		regroupData eddsaKeygen.LocalPartySaveData
		peerState   int
	}{
		{
			name:        "creating eddsa keygen data",
			regroupData: fixture,
			peerState:   1,
		},
		{
			name:        "creating eddsa keygen data",
			regroupData: fixture,
			peerState:   2,
		},
	}

	// using mocked structs and functions
	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetStorage").Return(store)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaRegroupOp := operationEDDSARegroup{
				OperationRegroup: OperationRegroup{
					LocalTssData: models.TssRegroupData{
						PeerState: tt.peerState,
					},
				},
			}
			err := eddsaRegroupOp.handleEndMessage(app, tt.regroupData)
			if err != nil {
				t.Errorf("handleEndMessage failed: %v", err)
			}
		})
	}
}

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
func TestEDDSA_gossipMessageHandler(t *testing.T) {
	// reding fixutre data
	fixture, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}

	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating localTssDatas and new partyIds
	partyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
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
	store := mockedStorage.NewStorage(t)
	app := mockedInterface.NewRosenTss(t)
	conn := mockedNetwork.NewConnection(t)

	tests := []struct {
		name         string
		regroupData  eddsaKeygen.LocalPartySaveData
		localTssData models.TssRegroupData
		tssMessage   tss.Message
		appConfig    func()
	}{
		{
			name:         "save regroup",
			regroupData:  fixture,
			localTssData: localTssData,
			appConfig: func() {
				store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(fmt.Errorf("message received"))
				app.On("GetPeerHome").Return("/tmp/.rosenTss")
				app.On("GetStorage").Return(store)
			},
		},
		{
			name:         "party message",
			localTssData: localTssData,
			tssMessage:   &message,
			appConfig: func() {
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
			partiesLength := len(localTssData.NewPartyIds) + len(localTssData.OldPartyIds)

			outCh := make(chan tss.Message, partiesLength)
			endCh := make(chan eddsaKeygen.LocalPartySaveData, partiesLength)
			switch tt.name {
			case "save regroup":
				endCh <- tt.regroupData
			case "party message":
				outCh <- tt.tssMessage
			}
			err := eddsaRegroupOp.gossipMessageHandler(app, outCh, endCh)
			if err != nil && err.Error() != "message received" {
				t.Errorf("gossipMessageHandler error = %v", err)
			}

		})
	}
}
