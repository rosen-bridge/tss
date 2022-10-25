package sign

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/binance-chain/tss-lib/common"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/blake2b"
	_interface "rosen-bridge/tss/app/interface"
	mockUtils "rosen-bridge/tss/mocks"
	mockedInterface "rosen-bridge/tss/mocks/app/interface"
	mocks "rosen-bridge/tss/mocks/app/sign"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
)

//------------------------ EDDSA tests ----------------------------

/*	TestEDDSA_registerMessageHandler
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
func TestEDDSASign_RegisterMessageHandler(t *testing.T) {

	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2),
	)

	// using mocked structures and functions
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)

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
				Crypto:      "eddsa",
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
				Crypto:      "eddsa",
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
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			localTssData: localTssData,
		},
	}

	logger, _ := mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage:  tt.signMessage,
					PeersMap:     make(map[string]string),
					Signatures:   make(map[string]string),
					Logger:       logger,
				}
				signerFunction := mockUtils.EDDSASigner(saveData)
				signer := mocks.NewSigner(t)
				signer.On("Sign", mock.AnythingOfType("[]byte")).Run(
					func(args mock.Arguments) ([]byte, err) {
						message := args.Get(0).([]byte)
						signed, err := signerFunction(message)
						if err != nil {
							return nil, err
						}
						return signed, nil
					},
				)

				// partyMessageHandler
				err := eddsaSignOp.RegisterMessageHandler(
					app,
					tt.gossipMessage,
					saveData.Ks,
					saveData.ShareID,
					mockUtils.EDDSASigner(saveData),
				)
				if err != nil {
					t.Error(err)
				}

			},
		)
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
func TestEDDSASign_PartyUpdate(t *testing.T) {

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
		append(localTssData.PartyIds.ToUnSorted(), newParty),
	)

	// creating new tss.Party for eddsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1,
	)
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

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	eddsaSignOp := OperationSign{
		LocalTssData: localTssData,
		SignMessage: models.SignMessage{
			Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			Crypto:      "eddsa",
			CallBackUrl: "http://localhost:5050/callback/sign",
		},
		Logger: logger,
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if err := eddsaSignOp.PartyUpdate(tt.message); (err != nil) != tt.wantErr {
					if !strings.Contains(err.Error(), "invalid wire-format data") {
						t.Errorf("PartyUpdate() error = %v, wantErr %v", err, tt.wantErr)
					}
				}

			},
		)
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
func TestEDDSASign_Setup(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetMetaData").Return(
		models.MetaData{
			PeersCount: 3,
			Threshold:  2,
		},
	)

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

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger: logger,
				}
				err := eddsaSignOp.Setup(tt.app, mockUtils.EDDSASigner(saveData))
				if err != nil && !tt.wantErr {
					t.Errorf("Setup error = %v", err)
				}

			},
		)
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
func TestEDDSASign_HandleOutMessage(t *testing.T) {
	// creating fake mockUtils.TestUtilsMessage as tss.Message
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating new localTssData
	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}

	// using mocked functions and structs
	app := mockedInterface.NewRosenTss(t)
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

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger: logger,
				}
				err := eddsaSignOp.HandleOutMessage(tt.app, tt.tssMessage, mockUtils.EDDSASigner(saveData))
				if err != nil {
					t.Errorf("handleOutMessage error = %v", err)
				}

			},
		)
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
func TestEDDSASign_HandleEndMessage(t *testing.T) {
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
	conn.On(
		"CallBack",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("models.SignData"),
		mock.AnythingOfType("string"),
	).Return(nil)
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

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := OperationSign{
					Logger: logger,
				}
				err := eddsaSignOp.HandleEndMessage(tt.app, tt.signatureData)
				if err != nil {
					t.Errorf("handleOutMessage error = %v", err)
				}

			},
		)
	}
}

/*	TestEDDSA_handleOutMessage
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
func TestEDDSASign_GossipMessageHandler(t *testing.T) {
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
	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	conn.On(
		"CallBack",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("models.SignData"),
		mock.AnythingOfType("string"),
	).Return(
		fmt.Errorf("message received"),
	)
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

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger: logger,
				}
				outCh := make(chan tss.Message, len(eddsaSignOp.LocalTssData.PartyIds))
				endCh := make(chan common.SignatureData, len(eddsaSignOp.LocalTssData.PartyIds))
				switch tt.name {

				case "handling sign":
					endCh <- *tt.signatureData
				case "party message":
					outCh <- tt.tssMessage
				}
				result, err := eddsaSignOp.GossipMessageHandler(tt.app, outCh, endCh, mockUtils.EDDSASigner(saveData))
				if err != nil {
					assert.Equal(t, result, false)
					if err.Error() != "message received" {
						t.Errorf("gossipMessageHandler error = %v", err)
					}
				} else {
					assert.Equal(t, result, true)
				}
			},
		)
	}
}

func TestEDDSASign_NewMessage(t *testing.T) {
	// creating localTssData and new partyId
	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	message := models.Payload{
		Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:   "cahj2pgs4eqvn1eo1tp0",
		ReceiverId: "",
		Name:       "register",
	}
	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
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

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := OperationSign{
					LocalTssData: localTssData,
					Logger:       logger,
				}
				err := eddsaSignOp.NewMessage(app, message, mockUtils.EDDSASigner(saveData))
				if err != nil && err.Error() != "message received" {
					t.Errorf("newMessage error = %v", err)
				}
			},
		)
	}
}

/*	TestEDDSASign_SetupMessageHandler
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
func TestEDDSASign_SetupMessageHandler(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	_, newPartyId, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	newPartyId2, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssDataWith2PartyIds := localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct

	msgBytes, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "eddsa", hex.EncodeToString(messageBytes[:]))

	setupMessage := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     tss.SortPartyIDs(append(tss.UnSortedPartyIDs{}, newPartyId2)),
		PeersMap:  make(map[string]string),
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId,
	}
	setupMessage.PeersMap[newPartyId.Id] = "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH"

	marshal, err := json.Marshal(setupMessage)
	if err != nil {
		t.Error(err)
	}
	gossipMessage := models.GossipMessage{
		Message:     string(marshal),
		MessageId:   messageId,
		SenderId:    newPartyId.Id,
		ReceiverId:  "",
		Name:        "setup",
		SenderP2PId: "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH",
	}

	tests := []struct {
		name         string
		appConfig    func() _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name: "new setup message",
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				index := utils.IndexOf(saveData.Ks, newPartyId.KeyInt())
				minutes := time.Now().Unix() / 60
				if minutes%int64(len(saveData.Ks)) == int64(index) {
					conn := mockedNetwork.NewConnection(t)
					conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
					app.On("GetConnection").Return(conn)
				}
				return app
			},
			localTssData: localTssData,
			wantErr:      true,
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					Logger:       logger,
				}
				err := eddsaSignOp.SetupMessageHandler(app, gossipMessage, saveData.Ks, mockUtils.EDDSASigner(saveData))
				if err != nil && !tt.wantErr {
					t.Errorf("Setup error = %v", err)
				}

			},
		)
	}
}

func TestEDDSASign_SignStarter(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	_, newPartyId, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	newPartyId2, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssDataWith2PartyIds := localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct

	msgBytes, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())

	setupMessage := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     tss.SortPartyIDs(append(tss.UnSortedPartyIDs{}, newPartyId2)),
		PeersMap:  make(map[string]string),
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId,
	}
	setupMessage.PeersMap[newPartyId.Id] = "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH"

	tests := []struct {
		name         string
		appConfig    func() _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name: "new setup message",
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				return app
			},
			localTssData: localTssData,
			wantErr:      true,
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				eddsaSignOp := OperationSign{
					LocalTssData:     tt.localTssData,
					Logger:           logger,
					SetupSignMessage: setupMessage,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				}
				err := eddsaSignOp.SignStarter(app, newPartyId.Id, mockUtils.EDDSASigner(saveData))
				if err != nil && !tt.wantErr {
					t.Errorf("Setup error = %v", err)
				}

			},
		)
	}
}

//------------------------ ECDSA tests ----------------------------

/*	TestECDSASign_registerMessageHandler
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
func TestECDSASign_RegisterMessageHandler(t *testing.T) {

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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2),
	)

	// using mocked structures and functions
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)

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

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage:  tt.signMessage,
					PeersMap:     make(map[string]string),
					Signatures:   make(map[string]string),
					Logger:       logger,
				}
				// partyIdMessageHandler
				err = ecdsaSignOp.RegisterMessageHandler(
					app,
					tt.gossipMessage,
					saveData.Ks,
					saveData.ShareID,
					mockUtils.ECDSASigner(saveData),
				)
				if err != nil {
					t.Error(err)
				}

			},
		)
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
func TestECDSASign_partyUpdate(t *testing.T) {

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
		append(localTssData.PartyIds.ToUnSorted(), newParty),
	)

	// creating new tss.Party for ecdsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.S256(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1,
	)
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

	logger, _ := mockUtils.InitLog("ecdsa-sign")
	ecdsaSignOp := OperationSign{
		LocalTssData: localTssData,
		SignMessage: models.SignMessage{
			Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			Crypto:      "ecdsa",
			CallBackUrl: "http://localhost:5050/callback/sign",
		},
		Logger: logger,
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if err := ecdsaSignOp.PartyUpdate(tt.message); (err != nil) != tt.wantErr {
					if !strings.Contains(err.Error(), "invalid wire-format data") {
						t.Errorf("PartyUpdate() error = %v, wantErr %v", err, tt.wantErr)
					}
				}

			},
		)
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
func TestECDSASign_setup(t *testing.T) {

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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetMetaData").Return(
		models.MetaData{
			PeersCount: 3,
			Threshold:  2,
		},
	)

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

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger: logger,
				}
				err := ecdsaSignOp.Setup(tt.app, mockUtils.ECDSASigner(saveData))
				if err != nil && !tt.wantErr {
					t.Errorf("Setup error = %v", err)
				}

			},
		)
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
func TestECDSASign_handleOutMessage(t *testing.T) {
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

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger: logger,
				}
				err := ecdsaSignOp.HandleOutMessage(tt.app, tt.tssMessage, mockUtils.ECDSASigner(saveData))
				if err != nil {
					t.Errorf("handleOutMessage error = %v", err)
				}

			},
		)
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
func TestECDSASign_handleEndMessage(t *testing.T) {
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
	conn.On(
		"CallBack",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("models.SignData"),
		mock.AnythingOfType("string"),
	).Return(nil)
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

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ecdsaSignOp := OperationSign{
					Logger: logger,
				}
				err := ecdsaSignOp.HandleEndMessage(tt.app, tt.signatureData)
				if err != nil {
					t.Errorf("handleOutMessage error = %v", err)
				}

			},
		)
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
func TestECDSASign_GossipMessageHandler(t *testing.T) {
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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	conn.On(
		"CallBack",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("models.SignData"),
		mock.AnythingOfType("string"),
	).Return(
		fmt.Errorf("message received"),
	)
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

	logger, err := mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger: logger,
				}
				outCh := make(chan tss.Message, len(ecdsaSignOp.LocalTssData.PartyIds))
				endCh := make(chan common.SignatureData, len(ecdsaSignOp.LocalTssData.PartyIds))
				switch tt.name {

				case "handling sign":
					endCh <- *tt.signatureData
				case "party message":
					outCh <- tt.tssMessage
				}
				result, err := ecdsaSignOp.GossipMessageHandler(tt.app, outCh, endCh, mockUtils.ECDSASigner(saveData))
				if err != nil {
					assert.Equal(t, result, false)
					if err.Error() != "message received" {
						t.Errorf("gossipMessageHandler error = %v", err)
					}
				} else {
					assert.Equal(t, result, true)
				}

			},
		)
	}
}

func TestECDSASign_NewMessage(t *testing.T) {
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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	message := models.Payload{
		Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:   "cahj2pgs4eqvn1eo1tp0",
		ReceiverId: "",
		Name:       "register",
	}
	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
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

	logger, err := mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ecdsaSignOp := OperationSign{
					LocalTssData: localTssData,
					Logger:       logger,
				}

				err := ecdsaSignOp.NewMessage(app, message, mockUtils.ECDSASigner(saveData))
				if err != nil && err.Error() != "message received" {
					t.Errorf("newMessage error = %v", err)
				}
			},
		)
	}
}

/*	TestECDSASign_SetupMessageHandler
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
func TestECDSASign_SetupMessageHandler(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}
	_, newPartyId, err := mockUtils.LoadECDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	newPartyId2, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssDataWith2PartyIds := localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct

	msgBytes, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "ecdsa", hex.EncodeToString(messageBytes[:]))

	setupMessage := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     tss.SortPartyIDs(append(tss.UnSortedPartyIDs{}, newPartyId2)),
		PeersMap:  make(map[string]string),
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId,
	}
	setupMessage.PeersMap[newPartyId.Id] = "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH"

	marshal, err := json.Marshal(setupMessage)
	if err != nil {
		t.Error(err)
	}
	gossipMessage := models.GossipMessage{
		Message:     string(marshal),
		MessageId:   messageId,
		SenderId:    newPartyId.Id,
		ReceiverId:  "",
		Name:        "setup",
		SenderP2PId: "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH",
	}

	tests := []struct {
		name         string
		appConfig    func() _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name: "new setup message",
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				index := utils.IndexOf(saveData.Ks, newPartyId.KeyInt())
				minutes := time.Now().Unix() / 60
				if minutes%int64(len(saveData.Ks)) == int64(index) {
					conn := mockedNetwork.NewConnection(t)
					conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
					app.On("GetConnection").Return(conn)
				}
				return app
			},
			localTssData: localTssData,
			wantErr:      true,
		},
	}

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					Logger:       logger,
				}
				err := ecdsaSignOp.SetupMessageHandler(app, gossipMessage, saveData.Ks, mockUtils.ECDSASigner(saveData))
				if err != nil && !tt.wantErr {
					t.Errorf("Setup error = %v", err)
				}

			},
		)
	}
}

func TestECDSASign_SignStarter(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalECDSAParty error = %v", err)
	}
	_, newPartyId, err := mockUtils.LoadECDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	newPartyId2, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssDataWith2PartyIds := localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct

	msgBytes, _ := hex.DecodeString("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())

	setupMessage := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     tss.SortPartyIDs(append(tss.UnSortedPartyIDs{}, newPartyId2)),
		PeersMap:  make(map[string]string),
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId,
	}
	setupMessage.PeersMap[newPartyId.Id] = "12D3KooWKT8NYknfaLUG3xtjNdtoTqVDxr1QrunghxSWWM8zdLrH"

	tests := []struct {
		name         string
		appConfig    func() _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name: "new setup message",
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				return app
			},
			localTssData: localTssData,
			wantErr:      true,
		},
	}

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				ecdsaSignOp := OperationSign{
					LocalTssData:     tt.localTssData,
					Logger:           logger,
					SetupSignMessage: setupMessage,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				}
				err := ecdsaSignOp.SignStarter(app, newPartyId.Id, mockUtils.ECDSASigner(saveData))
				if err != nil && !tt.wantErr {
					t.Errorf("Setup error = %v", err)
				}

			},
		)
	}
}
