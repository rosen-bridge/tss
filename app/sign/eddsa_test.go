package sign

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaSign "github.com/binance-chain/tss-lib/eddsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	mockUtils "rosen-bridge/tss/mocks"
	mockedInterface "rosen-bridge/tss/mocks/app/interface"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"strings"
	"testing"
	"time"
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
	storage := mockedStorage.NewStorage(t)
	storage.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(data, id, err)
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
			Crypto:     "eddsa",
			Name:       "partyId",
		})

	tests := []struct {
		name         string
		app          _interface.RosenTss
		receiverId   string
		localTssData models.TssData
	}{
		{
			name:         "creating partyId message with localTssData, there must be no error",
			app:          app,
			receiverId:   "",
			localTssData: localTssData,
		},
		{
			name:         "creating partyId message without localTssData, there must be no error",
			app:          app,
			receiverId:   "cahj2pgs4eqvn1eo1tp0",
			localTssData: models.TssData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := eddsaSignOp.Init(tt.app, tt.receiverId)
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
	saveData, Id1, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	_, Id2, err := mockUtils.LoadEDDSAKeygenFixture(1)
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
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID))
	signDataBytes, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(signDataBytes)

	// creating new tss party for eddsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))
	party := eddsaSign.NewLocalParty(signData, localTssData.Params, saveData, outCh, endCh)

	partyIDMessage := fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromSign")

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
					Crypto:     "eddsa",
					Name:       "partyId",
				},
			},
			AppConfig: func() {
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
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
					Crypto:     "eddsa",
					Name:       "partyMsg",
				},
			},
			AppConfig: func() {
				localTssData.Party = party
			},
		},
		{
			name:     "sign with party",
			expected: "handling incoming sign message from p2p, there must be no error out of err list",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    signData.String(),
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Crypto:     "eddsa",
					Name:       "sign",
				},
			},
			AppConfig: func() {
				localTssData.Party = party
			},
		},
		{
			name:     "sign without party",
			expected: "handling incoming sign message from p2p, there must be no error out of err list",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    signData.String(),
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Crypto:     "eddsa",
					Name:       "sign",
				},
			},
			AppConfig: func() {
				localTssData.Party = nil
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(models.GossipMessage{
						Message:    signData.String(),
						MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
						SenderId:   "cahj2pgs4eqvn1eo1tp0",
						ReceiverId: "",
						Crypto:     "eddsa",
						Name:       "sign",
					})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.AppConfig()
			eddsaSignOp := operationEDDSASign{
				savedData: saveData,
				OperationSign: OperationSign{
					LocalTssData: localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			messageCh := make(chan models.Message, 100)

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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{
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

/*	TestEDDSA_partyIdMessageHandler
	TestCases:
	testing message controller, there is 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.GossipMessage, models.SignMessage, models.TssData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetMetaData, GetConnection, NewMessage functions
*/
func TestEDDSA_partyIdMessageHandler(t *testing.T) {

	// creating new localTssData and new partyIds
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	newPartyId2, err := mockUtils.CreateNewEDDSAPartyId()
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
			Crypto:     "eddsa",
			Name:       "partyId",
		})

	tests := []struct {
		name          string
		gossipMessage models.GossipMessage
		signMessage   models.SignMessage
		localTssData  models.TssData
	}{
		{
			name: "partyId message with partyId list less than threshold, there must be no error",
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromSign"),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Crypto:     "eddsa",
				Name:       "partyId",
			},
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			localTssData: localTssData,
		},
		{
			name: "partyId message with partyId list equal to threshold, there must be no error",
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromSign"),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Crypto:     "eddsa",
				Name:       "partyId",
			},
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			localTssData: localTssDataWith2PartyIds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage:  tt.signMessage,
				},
			}
			// partyMessageHandler
			err := eddsaSignOp.partyIdMessageHandler(app, tt.gossipMessage)
			if err != nil {
				t.Error(err)
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
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
*/
func TestEDDSA_partyUpdate(t *testing.T) {

	// creating new localTssData and new partyId
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Error(err)
	}

	newParty, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newParty))

	// creating new tss.Party for eddsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.S256(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	localTssData.Party = eddsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

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

	eddsaSignOp := operationEDDSASign{
		OperationSign: OperationSign{
			LocalTssData: localTssData,
			SignMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := eddsaSignOp.partyUpdate(tt.message); (err != nil) != tt.wantErr {
				if !strings.Contains(err.Error(), "invalid wire-format data") {
					t.Errorf("PartyUpdate() error = %v, wantErr %v", err, tt.wantErr)
				}
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
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
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
			Crypto:     "eddsa",
			Name:       "partyId",
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := eddsaSignOp.setup(tt.app)
			if err != nil && !tt.wantErr {
				t.Errorf("Setup error = %v", err)
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
	// creating fake mockUtils.TestUtilsMessage as tss.Message
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating new localTssData
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}

	// using mocked functions and structs
	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Crypto:     "eddsa",
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
			name:         "creating party message from tss.Message received, there must be no error",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			err := eddsaSignOp.handleOutMessage(tt.app, tt.tssMessage)
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
	there are _interface.RosenTss, common.SignatureData used as test arguments.
	Dependencies:
	- network.CallBack function
	- rosenTss GetConnection functions
*/
func TestEDDSA_handleEndMessage(t *testing.T) {
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
	conn.On("CallBack", mock.AnythingOfType("string"), mock.AnythingOfType("models.SignData")).Return(nil)
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{},
			}
			err := eddsaSignOp.handleEndMessage(tt.app, tt.signatureData)
			if err != nil {
				t.Errorf("handleOutMessage error = %v", err)
			}

		})
	}
}

/*	TestEDDSA_handleOutMessage
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are _interface.RosenTss, models.TssData, tss.Message, common.SignatureData used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- network Publish, CallBack functions
	- rosenTss GetConnection, NewMessage functions
*/
func TestEDDSA_gossipMessageHandler(t *testing.T) {
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
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
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
			Crypto:     "eddsa",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	conn.On("CallBack", mock.AnythingOfType("string"), mock.AnythingOfType("models.SignData")).Return(
		fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)

	tests := []struct {
		name          string
		expected      string
		app           _interface.RosenTss
		signatureData common.SignatureData
		localTssData  models.TssData
		tssMessage    tss.Message
	}{
		{
			name:          "handling sign",
			expected:      "there should be an \"message received\" error",
			app:           app,
			signatureData: saveSign,
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaSignOp := operationEDDSASign{
				OperationSign: OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				},
			}
			outCh := make(chan tss.Message, len(eddsaSignOp.LocalTssData.PartyIds))
			endCh := make(chan common.SignatureData, len(eddsaSignOp.LocalTssData.PartyIds))
			switch tt.name {
			case "handling sign":
				endCh <- tt.signatureData
			case "party message":
				outCh <- tt.tssMessage
			}
			err := eddsaSignOp.gossipMessageHandler(tt.app, outCh, endCh)
			if err != nil && err.Error() != "message received" {
				t.Errorf("gossipMessageHandler error = %v", err)
			}

		})
	}
}