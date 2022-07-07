package ecdsa

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/keygen"
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
	- rosenTss GetStorage, GetConnection, GetPrivate, NewMessage functions
*/
func TestECDSA_Init(t *testing.T) {

	// creating localTssData
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssData
		appConfig    func() _interface.RosenTss
	}{
		{
			name:         "creating partyId message with localTssData",
			receiverId:   "",
			localTssData: localTssData,
			appConfig: func() _interface.RosenTss {
				// using mocked structs and functions
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("GetPrivate", mock.AnythingOfType("string")).Return("4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d", nil)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromKeygen"),
						MessageId:  "keygen",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
		},
		{
			name:         "creating partyId message without localTssData",
			receiverId:   "cahj2pgs4eqvn1eo1tp0",
			localTssData: models.TssData{},
			appConfig: func() _interface.RosenTss {
				// using mocked structs and functions
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("GetPrivate", mock.AnythingOfType("string")).Return("4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d", nil)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromKeygen"),
						MessageId:  "keygen",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
		},
		{
			name:         "creating partyId message without localTssData and without private",
			receiverId:   "cahj2pgs4eqvn1eo1tp0",
			localTssData: models.TssData{},
			appConfig: func() _interface.RosenTss {
				// using mocked structs and functions
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("GetPrivate", mock.AnythingOfType("string")).Return("", nil)
				app.On("SetPrivate", mock.AnythingOfType("models.Private")).Return(nil)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromKeygen"),
						MessageId:  "keygen",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Name:       "partyId",
					})
				return app
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.appConfig()
			ecdsaKeygenOp := operationECDSAKeygen{}
			err := ecdsaKeygenOp.Init(app, tt.receiverId)
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
	// creating localTssDatas and new partyIds
	_, Id1, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	_, Id2, err := mockUtils.LoadECDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	localTssData := models.TssData{
		PartyID: Id1,
	}
	newPartyId := Id2

	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID))

	// creating tss.Party for ecdsaKeygen
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan ecdsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	party := ecdsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	partyIDMessage := fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromKeygen")

	partyMessage := models.PartyMessage{
		Message:     []byte(partyIDMessage),
		IsBroadcast: true,
		GetFrom:     newPartyId,
		To:          []*tss.PartyID{localTssData.PartyID},
	}

	partyMessageBytes, err := json.Marshal(partyMessage)
	if err != nil {
		t.Error("failed to marshal message", err)
	}

	tests := []struct {
		name      string
		message   models.Message
		AppConfig func() _interface.RosenTss
	}{
		{
			name: "partyId",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    partyIDMessage,
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyId",
				},
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					})
				return app
			},
		},
		{
			name: "partyMsg",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    hex.EncodeToString(partyMessageBytes),
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyMsg",
				},
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				localTssData.Party = party
				return app
			},
		},
		{
			name: "keygen with party",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "generate key",
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "keygen",
				},
			},
			AppConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				localTssData.Party = party
				return app
			},
		},
		{
			name: "keygen without party",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "generate key",
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "keygen",
				},
			},
			AppConfig: func() _interface.RosenTss {
				localTssData.Party = nil
				app := mockedInterface.NewRosenTss(t)
				return app
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.AppConfig()
			ecdsaKeygenOp := operationECDSAKeygen{
				OperationKeygen: keygen.OperationKeygen{
					LocalTssData: localTssData,
				},
			}
			messageCh := make(chan models.Message, 1)

			messageCh <- tt.message
			go func() {
				time.Sleep(time.Second)
				close(messageCh)
			}()
			errorList := []string{"invalid wire-format", "channel closed"}
			err := ecdsaKeygenOp.Loop(app, messageCh)
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
			name:     "get ecdsa class name",
			expected: "ecdsaKeygen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaKeygenOp := operationECDSAKeygen{}

			result := ecdsaKeygenOp.GetClassName()
			if result != tt.expected {
				t.Errorf("GetClassName error = expected %s, got %s", tt.expected, result)
			}

		})
	}
}

/*	TestECDSA_partyIdMessageHandler
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
func TestECDSA_partyIdMessageHandler(t *testing.T) {
	// creating new localTssDatas and new partyIds
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

	tests := []struct {
		name          string
		gossipMessage models.GossipMessage
		localTssData  models.TssData
	}{
		{
			name: "partyId message with t=1",
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromKeygen"),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			localTssData: localTssData,
		},
		{
			name: "partyId message with t=2",
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromKeygen"),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			localTssData: localTssDataWith2PartyIds,
		},
	}

	// using mocked structs and functions
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tt.gossipMessage)
			ecdsaKeygenOp := operationECDSAKeygen{
				OperationKeygen: keygen.OperationKeygen{
					LocalTssData: tt.localTssData,
				},
			}
			// partyMessageHandler
			err := ecdsaKeygenOp.partyIdMessageHandler(app, tt.gossipMessage)
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
	- tss.Party for ecdsaKeygen.NewLocalParty
*/
func TestECDSA_partyUpdate(t *testing.T) {
	// creating localTssData and new partyId
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

	// creating new tss party for ecdsaKeygen
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
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
			name: "PartyUpdate from self to self",
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
			name: "PartyUpdate from new party to self",
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
			name: "PartyUpdate from new party to all",
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
			name: "PartyUpdate from self to all",
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

	ecdsaKeygenOp := operationECDSAKeygen{
		keygen.OperationKeygen{
			LocalTssData: localTssData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ecdsaKeygenOp.partyUpdate(tt.message); (err != nil) != tt.wantErr {
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

	// creating localTssData and new partyId
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

	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
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
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)
	tests := []struct {
		name         string
		app          _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name:         "creating keygen message partyId less than threshold",
			app:          app,
			localTssData: localTssData,
			wantErr:      true,
		},
		{
			name:         "creating keygen message with partyId equal to threshold",
			app:          app,
			localTssData: localTssDataWith2PartyIds,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaKeygenOp := operationECDSAKeygen{
				keygen.OperationKeygen{
					LocalTssData: tt.localTssData,
				},
			}
			err := ecdsaKeygenOp.setup(tt.app)
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
	app.On("GetConnection").Return(conn)
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
			ecdsaKeygenOp := operationECDSAKeygen{
				keygen.OperationKeygen{
					LocalTssData: tt.localTssData,
				},
			}
			err := ecdsaKeygenOp.handleOutMessage(tt.app, tt.tssMessage)
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
		name       string
		keygenData ecdsaKeygen.LocalPartySaveData
	}{
		{name: "creating ecdsa keygen data", keygenData: fixture},
	}

	// using mocked structs and functions
	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	ecdsaKeygenOp := operationECDSAKeygen{}

	app := mockedInterface.NewRosenTss(t)
	app.On("GetStorage").Return(store)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ecdsaKeygenOp.handleEndMessage(app, tt.keygenData)
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
	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("message received"))
	app := mockedInterface.NewRosenTss(t)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)
	app.On("GetStorage").Return(store)
	tests := []struct {
		name         string
		app          _interface.RosenTss
		keygenData   ecdsaKeygen.LocalPartySaveData
		localTssData models.TssData
		tssMessage   tss.Message
	}{
		{
			name:         "save keygen",
			app:          app,
			keygenData:   fixture,
			localTssData: localTssData,
		},
		{
			name:         "party message",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaKeygenOp := operationECDSAKeygen{
				keygen.OperationKeygen{LocalTssData: tt.localTssData},
			}
			outCh := make(chan tss.Message, len(ecdsaKeygenOp.LocalTssData.PartyIds))
			endCh := make(chan ecdsaKeygen.LocalPartySaveData, len(ecdsaKeygenOp.LocalTssData.PartyIds))
			switch tt.name {
			case "save keygen":
				endCh <- tt.keygenData
			case "party message":
				outCh <- tt.tssMessage
			}
			err := ecdsaKeygenOp.gossipMessageHandler(tt.app, outCh, endCh)
			if err != nil && err.Error() != "message received" {
				t.Errorf("gossipMessageHandler error = %v", err)
			}

		})
	}
}
