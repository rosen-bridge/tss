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
	eddsaSign "github.com/binance-chain/tss-lib/eddsa/signing"
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

/*	TestEDDSASign_SignStarterThread
	TestCases:
	testing SignStarterThread, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.SetupSign used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetConfig functions
*/
func TestSign_SignStarterThread(t *testing.T) {

	// creating new localTssData and new partyId
	_, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
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
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)
	localTssDataWithallPeer := localTssData
	localTssDataWithallPeer.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2),
	)

	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1,
	)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	party := eddsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	// using mocked function and struct
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())

	setupSignMsg := models.SetupSign{
		Hash:      utils.Encoder(messageBytes[:]),
		Peers:     []tss.PartyID{*newPartyId2},
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	tests := []struct {
		name         string
		appConfig    func() (_interface.RosenTss, Handler)
		localTssData models.TssData
	}{
		{
			name: "new setup message, localPartiIds does not have all of the setup message peers",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:         60,
						SignStartTimeTracker: 1,
					},
				)
				handler := mocks.NewHandler(t)
				return app, handler
			},
			localTssData: localTssData,
		},
		{
			name: "new setup message, localPartiIds has all of the setup message peers",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:         60,
						SignStartTimeTracker: 1,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))

				return app, handler
			},
			localTssData: localTssDataWithallPeer,
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, subtest := range tests {
		tt := subtest
		t.Run(
			tt.name, func(t *testing.T) {
				app, handler := tt.appConfig()

				eddsaSignOp := OperationSign{
					LocalTssData:     tt.localTssData,
					Logger:           logger,
					SetupSignMessage: setupSignMsg,
					SignMessage:      signMsg,
					Handler:          handler,
				}

				ticker := time.NewTicker(time.Second * time.Duration(2))
				done := make(chan bool)

				go func() {
					for {
						select {
						case <-done:
							return
						case <-ticker.C:
							eddsaSignOp.LocalTssData.Party = party
							done <- true
						}
					}
				}()

				eddsaSignOp.SignStarterThread(app)
			},
		)
	}
}

/*	TestSign_SetupThread
	TestCases:
	testing SignStarterThread, there are 3 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- localTssData models.TssData
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- rosenTss GetConfig functions
	- handler GetData, Sign functions
*/
func TestSign_SetupThread(t *testing.T) {

	// creating new localTssData and new partyId
	saveData1, pId1, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	saveData2, pId2, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	saveData3, pId3, err := mockUtils.LoadEDDSAKeygenFixture(2)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData := models.TssData{
		PartyID: pId1,
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), pId1),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), pId2),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), pId3),
	)
	localTssData2 := localTssData
	localTssData3 := localTssData
	localTssData2.PartyID = pId2
	localTssData3.PartyID = pId3

	// using mocked function and struct
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	tests := []struct {
		name         string
		appConfig    func() (_interface.RosenTss, Handler)
		localTssData models.TssData
		saveData     eddsaKeygen.LocalPartySaveData
	}{
		{
			name: "setup with pid1, there must be no unexpected error",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    3,
					},
				)
				handler := mocks.NewHandler(t)

				index := int64(utils.IndexOf(saveData1.Ks, saveData1.ShareID))
				length := int64(len(saveData1.Ks))
				round := time.Now().Unix() / 60
				if round%length == index && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData1.Ks, saveData1.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))
				}

				return app, handler
			},
			localTssData: localTssData,
			saveData:     saveData1,
		},
		{
			name: "setup with pid2, there must be no unexpected error",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    3,
					},
				)
				handler := mocks.NewHandler(t)

				index := int64(utils.IndexOf(saveData2.Ks, saveData2.ShareID))
				length := int64(len(saveData2.Ks))
				round := time.Now().Unix() / 60
				if round%length == index && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData2.Ks, saveData2.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))
				}

				return app, handler
			},
			localTssData: localTssData2,
			saveData:     saveData2,
		},
		{
			name: "setup with pid3, there must be no unexpected error",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    3,
					},
				)
				handler := mocks.NewHandler(t)

				index := int64(utils.IndexOf(saveData3.Ks, saveData3.ShareID))
				length := int64(len(saveData3.Ks))
				round := time.Now().Unix() / 60
				if round%length == index && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData3.Ks, saveData3.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))
				}

				return app, handler
			},
			localTssData: localTssData3,
			saveData:     saveData3,
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		time.Sleep(1 * time.Second)
		t.Run(
			tt.name, func(t *testing.T) {
				//if deadline, ok := t.Deadline(); ok {
				//	t.Log(deadline)
				//timeout := time.Until(deadline)
				app, handler := tt.appConfig()

				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					Logger:       logger,
					SignMessage:  signMsg,
					Handler:      handler,
				}

				ticker := time.NewTicker(time.Second * time.Duration(1))

				go func() {
					for {
						select {
						case <-ticker.C:
							return
						default:
							err := eddsaSignOp.SetupThread(app, tt.saveData.Ks, tt.saveData.ShareID, 2)
							if err != nil && err.Error() != "can not sign" {
								t.Error(err)
							}
						}
					}
				}()
				time.Sleep(1 * time.Second)
			},
		)
	}
}

/*	TestSign_CreateParty
	TestCases:
	testing SignStarterThread, there is 1 testcase.
	based on local data the process should create and start party
	there must be no error
*/
func TestSign_CreateParty(t *testing.T) {
	// pre-test part, faking data and using mocks

	// reading eddsaKeygen.LocalPartySaveData from fixtures
	_, Id1, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	_, Id2, err := mockUtils.LoadEDDSAKeygenFixture(2)
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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), Id3),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID),
	)
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	app := mockedInterface.NewRosenTss(t)
	app.On("GetMetaData").Return(
		models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
	)
	handler := mocks.NewHandler(t)
	handler.On(
		"StartParty",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(fmt.Errorf("party started"))

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "create party with given data",
			expected: "there must be no error",
		},
	}
	var peers []tss.PartyID
	for _, peer := range localTssData.PartyIds {
		peers = append(peers, *peer)
	}
	errorCh := make(chan error)

	logger, _ := mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := OperationSign{
					LocalTssData: localTssData,
					Logger:       logger,
					SignMessage:  signMsg,
					Handler:      handler,
				}

				go func() {
					for {
						select {
						case err := <-errorCh:
							if err.Error() != "party started" {
								t.Error(err)
							}
						}
					}
				}()
				eddsaSignOp.CreateParty(app, peers, errorCh)

			},
		)
	}
}

/*	TestSign_SignMessageHandler
	TestCases:
	testing SignStarterThread, there are 9 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- localTssData models.TssData
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- rosenTss GetConfig, GetMetaData functions
	- handler GetData, Sign, Verify functions
*/
func TestSign_SignMessageHandler(t *testing.T) {

	// creating new localTssData and new partyId
	newSenderId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	saveData, newPartyId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData := models.TssData{
		PartyID: newPartyId,
	}
	saveData2, newPartyId2, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	saveData3, newPartyId3, err := mockUtils.LoadEDDSAKeygenFixture(2)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId3),
	)
	localTssData2 := localTssData
	localTssData3 := localTssData
	localTssData2.PartyID = newPartyId2
	localTssData3.PartyID = newPartyId3

	// using mocked function and struct
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())

	var peers []tss.PartyID
	for _, peer := range localTssData.PartyIds {
		peers = append(peers, *peer)
	}

	setupSignMsg := models.SetupSign{
		Hash:      utils.Encoder(signDataBytes[:]),
		Peers:     peers,
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	marshal, err := json.Marshal(setupSignMsg)
	if err != nil {
		t.Error(err)
	}
	signature1, err := mockUtils.EDDSASigner(saveData)(marshal)
	if err != nil {
		t.Error(err)
	}
	signature2, err := mockUtils.EDDSASigner(saveData2)(marshal)
	if err != nil {
		t.Error(err)
	}
	signature3, err := mockUtils.EDDSASigner(saveData3)(marshal)
	if err != nil {
		t.Error(err)
	}
	messageId := fmt.Sprintf("%s%s", signMsg.Crypto, utils.Encoder(signDataBytes[:]))
	index, _ := saveData.OriginalIndex()
	index2, _ := saveData2.OriginalIndex()
	index3, _ := saveData3.OriginalIndex()

	tests := []struct {
		name          string
		appConfig     func() (_interface.RosenTss, Handler)
		localTssData  models.TssData
		gossipMsg     models.GossipMessage
		saveData      eddsaKeygen.LocalPartySaveData
		expectedError error
		signatures    map[int][]byte
	}{
		{
			name: "new sign message, can not be verified",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("can not verify"))

				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      signMessage,
				Index:     index,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("can not verify"),
			signatures:    make(map[int][]byte),
		},
		{
			name: "new sign message, sender not exist in the list of peers",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  newSenderId.Id,
				Name:      signMessage,
				Index:     index,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("sender not exist in the list of peers"),
			signatures:    make(map[int][]byte),
		},
		{
			name: "new sign message, signatures are less than threshold",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      signMessage,
				Index:     index,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf(""),
			signatures:    make(map[int][]byte),
		},
		{
			name: "new sign message, signatures are equal to threshold for peer 1, returns error at message signing",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				length := int64(len(saveData.Ks))
				round := time.Now().Unix() / 60

				if round%length == int64(index) && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))
				}
				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      signMessage,
				Index:     index,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("can not sign"),
			signatures:    map[int][]byte{index2: signature2},
		},
		{
			name: "new sign message, signatures are equal to threshold for peer 2, returns error at message signing",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				length := int64(len(saveData2.Ks))
				round := time.Now().Unix() / 60

				if round%length == int64(index2) && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData2.Ks, saveData2.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))
				}
				return app, handler
			},
			localTssData: localTssData2,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      signMessage,
				Index:     index2,
			},
			saveData:      saveData2,
			expectedError: fmt.Errorf("can not sign"),
			signatures:    map[int][]byte{index: signature1},
		},
		{
			name: "new sign message, signatures are equal to threshold for peer 3, returns error at message signing",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				length := int64(len(saveData3.Ks))
				round := time.Now().Unix() / 60

				if round%length == int64(index3) && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData3.Ks, saveData3.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, fmt.Errorf("can not sign"))
				}
				return app, handler
			},
			localTssData: localTssData3,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature2),
				MessageId: messageId,
				SenderId:  localTssData2.PartyID.Id,
				Name:      signMessage,
				Index:     index3,
			},
			saveData:      saveData3,
			expectedError: fmt.Errorf("can not sign"),
			signatures:    map[int][]byte{index: signature1},
		},
		{
			name: "new sign message, signatures are equal to threshold for peer 1, returns error at creating party",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				length := int64(len(saveData.Ks))
				round := time.Now().Unix() / 60

				if round%length == int64(index) && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, nil)
					handler.On(
						"StartParty",
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
					).Return(fmt.Errorf("party started"))
					conn := mockedNetwork.NewConnection(t)
					conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
					app.On("GetConnection").Return(conn)
				}
				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      signMessage,
				Index:     index,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("party started"),
			signatures:    map[int][]byte{index2: signature2},
		},
		{
			name: "new sign message, signatures are equal to threshold for peer 2, returns error at creating party",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				length := int64(len(saveData2.Ks))
				round := time.Now().Unix() / 60

				if round%length == int64(index2) && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData2.Ks, saveData2.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, nil)
					handler.On(
						"StartParty",
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
					).Return(fmt.Errorf("party started"))
					conn := mockedNetwork.NewConnection(t)
					conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
					app.On("GetConnection").Return(conn)
				}
				return app, handler
			},
			localTssData: localTssData2,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature3),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      signMessage,
				Index:     index2,
			},
			saveData:      saveData2,
			expectedError: fmt.Errorf("party started"),
			signatures:    map[int][]byte{index: signature1},
		},
		{
			name: "new sign message, signatures are equal to threshold for peer 3, returns error at creating party",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				length := int64(len(saveData3.Ks))
				round := time.Now().Unix() / 60

				if round%length == int64(index3) && int64(time.Now().Second()) < 50 {
					handler.On("GetData").Return(saveData3.Ks, saveData3.ShareID)
					handler.On("Sign", mock.Anything).Return([]byte{}, nil)
					handler.On(
						"StartParty",
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
					).Return(fmt.Errorf("party started"))
					conn := mockedNetwork.NewConnection(t)
					conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
					app.On("GetConnection").Return(conn)
				}
				return app, handler
			},
			localTssData: localTssData3,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(signature2),
				MessageId: messageId,
				SenderId:  localTssData2.PartyID.Id,
				Name:      signMessage,
				Index:     index3,
			},
			saveData:      saveData3,
			expectedError: fmt.Errorf("party started"),
			signatures:    map[int][]byte{index: signature1},
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app, handler := tt.appConfig()
				errorCh := make(chan error)

				eddsaSignOp := OperationSign{
					LocalTssData:         tt.localTssData,
					Logger:               logger,
					SelfSetupSignMessage: setupSignMsg,
					SignMessage:          signMsg,
					Handler:              handler,
					Signatures:           tt.signatures,
				}

				go func() {
					for {
						select {
						case err := <-errorCh:
							if err.Error() != "party started" {
								t.Error(err)
							}
						}
					}
				}()

				err := eddsaSignOp.SignMessageHandler(app, tt.gossipMsg, tt.saveData.Ks, tt.saveData.ShareID, errorCh)
				if err != nil && err.Error() != tt.expectedError.Error() {
					t.Errorf("SignMessageHandler error = %v", err)
				}

			},
		)
	}
}

/*	TestSign_StartSignMessageHandler
	TestCases:
	testing SignStarterThread, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- localTssData models.TssData
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- rosenTss GetConfig, GetMetaData functions
	- handler GetData, Sign, Verify functions
*/
func TestSign_StartSignMessageHandler(t *testing.T) {

	// creating new localTssData and new partyId
	newSenderId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	saveData, newPartyId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData := models.TssData{
		PartyID: newPartyId,
	}
	saveData2, newPartyId2, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	_, newPartyId3, err := mockUtils.LoadEDDSAKeygenFixture(2)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId3),
	)
	localTssData2 := localTssData
	localTssData3 := localTssData
	newLocalTssData := localTssData
	localTssData2.PartyID = newPartyId2
	localTssData3.PartyID = newPartyId3
	newLocalTssData.PartyID = newSenderId

	t.Log(newSenderId.Id)
	t.Log(newPartyId.Id)
	t.Log(newPartyId2.Id)
	t.Log(newPartyId3.Id)

	// using mocked function and struct
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())

	var peers []tss.PartyID
	for _, peer := range localTssData.PartyIds {
		peers = append(peers, *peer)
	}

	setupSignMsg := models.SetupSign{
		Hash:      utils.Encoder(signDataBytes[:]),
		Peers:     peers,
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	marshal, err := json.Marshal(setupSignMsg)
	if err != nil {
		t.Error(err)
	}
	signature1, err := mockUtils.EDDSASigner(saveData)(marshal)
	if err != nil {
		t.Error(err)
	}
	signature2, err := mockUtils.EDDSASigner(saveData2)(marshal)
	if err != nil {
		t.Error(err)
	}

	messageId := fmt.Sprintf("%s%s", signMsg.Crypto, utils.Encoder(signDataBytes[:]))
	index, _ := saveData.OriginalIndex()
	index2, _ := saveData2.OriginalIndex()

	startSignWithWrongHash := models.StartSign{
		Hash: utils.Encoder([]byte{12, 45, 32}),
	}
	startSignWithWrongHashMarshaled, err := json.Marshal(startSignWithWrongHash)
	if err != nil {
		t.Error(err)
	}
	round := time.Now().Unix() / 60

	startSignWithTrueHash := models.StartSign{
		Hash:       utils.Encoder(signDataBytes[:]),
		Peers:      peers,
		Timestamp:  round,
		StarterId:  newPartyId3.Id,
		Signatures: map[int][]byte{index2: signature2, index: signature1},
	}
	startSignWithTrueHashMarshaled, err := json.Marshal(startSignWithTrueHash)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name          string
		appConfig     func() (_interface.RosenTss, Handler)
		localTssData  models.TssData
		gossipMsg     models.GossipMessage
		saveData      eddsaKeygen.LocalPartySaveData
		expectedError error
		signatures    map[int][]byte
	}{
		{
			name: "start sign message with wrong hash ",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				handler := mocks.NewHandler(t)
				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(startSignWithWrongHashMarshaled),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      startSignMessage,
				Index:     index,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("wrong hash to sign"),
			signatures:    make(map[int][]byte),
		},
		{
			name: "start sign message it's not peer turn ",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				return app, handler
			},
			localTssData: localTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(startSignWithTrueHashMarshaled),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      startSignMessage,
				Index:     int(round%int64(len(saveData.Ks))) + 1,
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("turn"),
			signatures:    make(map[int][]byte),
		},
		{
			name: "start sign message this party is not part of signing process",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				handler := mocks.NewHandler(t)
				return app, handler
			},
			localTssData: newLocalTssData,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(startSignWithTrueHashMarshaled),
				MessageId: messageId,
				SenderId:  localTssData3.PartyID.Id,
				Name:      startSignMessage,
				Index:     int(round % int64(len(saveData.Ks))),
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("party is not part of signing process"),
			signatures:    make(map[int][]byte),
		},
		{
			name: "start sign message",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
					},
				)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				handler := mocks.NewHandler(t)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				return app, handler
			},
			localTssData: localTssData3,
			gossipMsg: models.GossipMessage{
				Message:   utils.Encoder(startSignWithTrueHashMarshaled),
				MessageId: messageId,
				SenderId:  localTssData2.PartyID.Id,
				Name:      startSignMessage,
				Index:     int(round % int64(len(saveData.Ks))),
			},
			saveData:      saveData,
			expectedError: fmt.Errorf("party is not part of signing process"),
			signatures:    make(map[int][]byte),
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app, handler := tt.appConfig()

				eddsaSignOp := OperationSign{
					LocalTssData:         tt.localTssData,
					Logger:               logger,
					SelfSetupSignMessage: setupSignMsg,
					SignMessage:          signMsg,
					Handler:              handler,
					Signatures:           tt.signatures,
				}

				_, err := eddsaSignOp.StartSignMessageHandler(app, tt.gossipMsg, tt.saveData.Ks)
				if err != nil && !strings.Contains(err.Error(), tt.expectedError.Error()) {
					t.Errorf("StartSignMessageHandler error = %v", err)
				}

			},
		)
	}
}

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

	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	tests := []struct {
		name         string
		noAnswer     bool
		senderId     string
		appConfig    func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler)
		localTssData models.TssData
	}{
		{
			name:         "register message with one party in partyId list with no answer false",
			noAnswer:     false,
			senderId:     newPartyId2.Id,
			localTssData: localTssData,
			appConfig: func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler) {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return(gossipMessage.Signature, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				return app, handler
			},
		},
		{
			name:         "register message with 2 partyId in list with no answer true and senderId equals to localPartyId",
			noAnswer:     true,
			senderId:     newPartyId.Id,
			localTssData: localTssDataWith2PartyIds,
			appConfig: func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				return app, nil
			},
		},
		{
			name:         "register message with 2 partyId in list with no answer true and senderId not equals to localPartyId",
			noAnswer:     true,
			senderId:     newPartyId2.Id,
			localTssData: localTssDataWith2PartyIds,
			appConfig: func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				return app, nil
			},
		},
	}

	logger, _ := mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				registerMsg := models.Register{
					Id:        newPartyId.Id,
					Moniker:   newPartyId.Moniker,
					Key:       newPartyId.KeyInt().String(),
					Timestamp: time.Now().Unix() / 60,
					NoAnswer:  tt.noAnswer,
				}
				marshal, err := json.Marshal(registerMsg)
				if err != nil {
					return
				}
				payload := mockUtils.CreatePayload(marshal, tt.senderId, signMsg, registerMessage)
				gossipMessage, err := mockUtils.CreateGossipMessageForEDDSA(payload, "", saveData)
				if err != nil {
					t.Errorf(err.Error())
				}
				app, handler := tt.appConfig(gossipMessage)
				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage:  signMsg,
					Signatures:   make(map[int][]byte),
					Logger:       logger,
					Handler:      handler,
				}

				// partyMessageHandler
				err = eddsaSignOp.RegisterMessageHandler(app, gossipMessage)
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
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetConfig").Return(
		models.Config{TurnDuration: 60},
	)
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	tests := []struct {
		name         string
		app          _interface.RosenTss
		setupSignMsg models.SetupSign
	}{
		{
			name:         "creating setup message with self setup is empty",
			app:          app,
			setupSignMsg: models.SetupSign{},
		},
		{
			name: "creating setup message with self setup is not empty",
			app:  app,
			setupSignMsg: models.SetupSign{
				Hash: signMsg.Message,
			},
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				eddsaSignOp := OperationSign{
					LocalTssData:         localTssData,
					SignMessage:          signMsg,
					Logger:               logger,
					Handler:              handler,
					SelfSetupSignMessage: tt.setupSignMsg,
				}
				err := eddsaSignOp.Setup(tt.app)
				if err != nil {
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
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger:  logger,
					Handler: handler,
				}
				err := eddsaSignOp.HandleOutMessage(tt.app, tt.tssMessage)
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

	tests := []struct {
		name          string
		expected      string
		appConfig     func() (_interface.RosenTss, Handler)
		signatureData *common.SignatureData
		tssMessage    tss.Message
	}{
		{
			name:          "handling sign",
			expected:      "there should be an \"message received\" error",
			signatureData: &saveSign,
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On(
					"CallBack",
					mock.AnythingOfType("string"),
					mock.AnythingOfType("models.SignData"),
					mock.AnythingOfType("string"),
				).Return(
					fmt.Errorf("message received"),
				)
				app.On("GetConnection").Return(conn)

				handler := mocks.NewHandler(t)

				return app, handler
			},
		},
		{
			name:       "party message",
			expected:   "there should be an \"message received\" error",
			tssMessage: &message,
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
				app.On("GetConnection").Return(conn)

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				return app, handler
			},
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				app, handler := tt.appConfig()

				eddsaSignOp := OperationSign{
					LocalTssData: localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger:  logger,
					Handler: handler,
				}
				outCh := make(chan tss.Message, len(eddsaSignOp.LocalTssData.PartyIds))
				endCh := make(chan common.SignatureData, len(eddsaSignOp.LocalTssData.PartyIds))
				switch tt.name {

				case "handling sign":
					endCh <- *tt.signatureData
				case "party message":
					outCh <- tt.tssMessage
				}
				result, err := eddsaSignOp.GossipMessageHandler(app, outCh, endCh)
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
		Message:   "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		MessageId: "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:  "cahj2pgs4eqvn1eo1tp0",
		Name:      "register",
	}
	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)
	handler := mocks.NewHandler(t)
	handler.On("Sign", mock.Anything).Return([]byte{}, nil)
	handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

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
					Handler:      handler,
				}
				err := eddsaSignOp.NewMessage(app, message, "")
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

	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	msgBytes, _ := hex.DecodeString(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "eddsa", hex.EncodeToString(messageBytes[:]))

	setupMsg := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     []tss.PartyID{*newPartyId2},
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	marshal, err := json.Marshal(setupMsg)
	if err != nil {
		t.Error(err)
	}
	gossipMessage := models.GossipMessage{
		Message:    utils.Encoder(marshal),
		MessageId:  messageId,
		SenderId:   newPartyId.Id,
		ReceiverId: "",
		Name:       "setup",
	}

	tests := []struct {
		name      string
		appConfig func() (_interface.RosenTss, Handler)
		index     int
	}{
		{
			name: "new setup message, round equals to index",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{TurnDuration: 60},
				)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				return app, handler
			},
			index: int((time.Now().Unix() / 60) % int64(len(saveData.Ks))),
		},
		{
			name: "new setup message, round not equals to index",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{TurnDuration: 60},
				)
				return app, nil
			},
			index: int((time.Now().Unix()/60)%int64(len(saveData.Ks))) + 1,
		},
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app, handler := tt.appConfig()
				eddsaSignOp := OperationSign{
					LocalTssData: localTssData,
					Logger:       logger,
					SignMessage:  signMsg,
					Handler:      handler,
				}
				gossipMessage.Index = tt.index
				err := eddsaSignOp.SetupMessageHandler(app, gossipMessage, saveData.Ks)
				if err != nil {
					t.Errorf("SetupMessageHandler error = %v", err)
				}

			},
		)
	}
}

/*	TestEDDSASign_SignStarter
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.GossipMessage, models.SignMessage, models.SetupSign used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetConnection functions
	- handler GetData, Sign functions
*/
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
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())

	setupSignMsg := models.SetupSign{
		Hash:      utils.Encoder(messageBytes[:]),
		Peers:     []tss.PartyID{*newPartyId2},
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	tests := []struct {
		name         string
		appConfig    func() _interface.RosenTss
		localTssData models.TssData
	}{
		{
			name: "creating new sign message",
			appConfig: func() _interface.RosenTss {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				return app
			},
			localTssData: localTssData,
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
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				eddsaSignOp := OperationSign{
					LocalTssData:     tt.localTssData,
					Logger:           logger,
					SetupSignMessage: setupSignMsg,
					SignMessage:      signMsg,
					Handler:          handler,
				}
				err := eddsaSignOp.SignStarter(app)
				if err != nil {
					t.Errorf("SignStarter error = %v", err)
				}

			},
		)
	}
}

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

	// using mock functions

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssData
		appConfig    func() _interface.RosenTss
	}{
		{
			name: "initiate partyId and creating register message",
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
	}

	logger, err := mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("LoadData", mock.Anything).Return(
					tss.NewPartyID("3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD", "", big.NewInt(10)),
					nil,
				)

				eddsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger:  logger,
					Handler: handler,
				}

				err := eddsaSignOp.Init(app, tt.receiverId)
				if err != nil {
					t.Errorf("Init failed: %v", err)
				}

			},
		)
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
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), Id3),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID),
	)
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "eddsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	signDataBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(signDataBytes)

	// creating new tss party for eddsa sign
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1,
	)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))
	party := eddsaSign.NewLocalParty(signData, localTssData.Params, saveData, outCh, endCh)

	registerMsg := models.Register{
		Id:        newPartyId.Id,
		Moniker:   newPartyId.Moniker,
		Key:       newPartyId.KeyInt().String(),
		Timestamp: time.Now().Unix() / 60,
		NoAnswer:  false,
	}
	marshal, err := json.Marshal(registerMsg)
	if err != nil {
		t.Errorf("registerMessage error = %v", err)
	}

	partyMsg := models.PartyMessage{
		Message:     marshal,
		IsBroadcast: true,
		GetFrom:     newPartyId,
		To:          []*tss.PartyID{localTssData.PartyID},
	}

	partyMessageBytes, err := json.Marshal(partyMsg)
	if err != nil {
		t.Error("failed to marshal message", err)
	}

	messageBytes := blake2b.Sum256(signData.Bytes())
	var peers []tss.PartyID
	for _, peer := range localTssData.PartyIds {
		peers = append(peers, *peer)
	}
	setupSignMsg := models.SetupSign{
		Hash:      utils.Encoder(messageBytes[:]),
		Peers:     peers,
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}
	setupMessageMarshaled, err := json.Marshal(setupSignMsg)
	if err != nil {
		t.Errorf("setupMessageMarshaled error = %v", err)
	}
	round := time.Now().Unix() / 60
	index := int(round % int64(len(saveData.Ks)))

	startSign := models.StartSign{
		Hash:  utils.Encoder(messageBytes[:]),
		Peers: peers,
	}
	startSignMarshaled, err := json.Marshal(startSign)
	if err != nil {
		t.Error(err)
	}

	// test cases
	tests := []struct {
		name      string
		expected  string
		message   models.GossipMessage
		AppConfig func() (_interface.RosenTss, Handler)
	}{
		{
			name:     "register",
			expected: "handling incoming register message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    utils.Encoder(marshal),
				MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       registerMessage,
				Index:      index,
			},
			AppConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    10,
						SignStartTimeTracker:      1,
					},
				)

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
		},
		{
			name:     "register returns error",
			expected: "handling incoming register message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    utils.Encoder(marshal),
				MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       registerMessage,
				Index:      index,
			},
			AppConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("close channel"))
				app.On("GetConnection").Return(conn)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    10,
						SignStartTimeTracker:      1,
					},
				)

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
		},
		{
			name:     "setup",
			expected: "handling incoming setup message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    utils.Encoder(setupMessageMarshaled),
				MessageId:  "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       setupMessage,
				Index:      index,
			},
			AppConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    10,
						SignStartTimeTracker:      1,
					},
				)

				handler := mocks.NewHandler(t)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
		},
		{
			name:     "partyMsg",
			expected: "handling incoming partyMsg message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    utils.Encoder(partyMessageBytes),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       partyMessage,
				Index:      index,
			},
			AppConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				localTssData.Party = party

				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    10,
						SignStartTimeTracker:      1,
					},
				)

				handler := mocks.NewHandler(t)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
		},
		{
			name:     "start sign",
			expected: "handling incoming sign message from p2p, there must be no error out of err list",
			message: models.GossipMessage{
				Message:    utils.Encoder(startSignMarshaled),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   newPartyId.Id,
				ReceiverId: "",
				Name:       startSignMessage,
				Index:      index,
			},
			AppConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)

				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					},
				)
				app.On("GetConfig").Return(
					models.Config{
						TurnDuration:              60,
						LeastProcessRemainingTime: 50,
						SetupBroadcastInterval:    10,
						SignStartTimeTracker:      1,
					},
				)

				handler := mocks.NewHandler(t)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				return app, handler
			},
		},
	}

	logger, _ := mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app, handler := tt.AppConfig()

				eddsaSignOp := OperationSign{
					LocalTssData: localTssData,
					SignMessage:  signMsg,
					Signatures:   make(map[int][]byte),
					Logger:       logger,
					Handler:      handler,
				}

				messageCh := make(chan models.GossipMessage, 100)

				payload := models.Payload{
					Message:   tt.message.Message,
					MessageId: tt.message.MessageId,
					SenderId:  tt.message.SenderId,
					Name:      tt.message.Name,
				}
				marshal, _ = json.Marshal(payload)
				eddsaSigner := mockUtils.EDDSASigner(saveData2)
				signature, _ := eddsaSigner(marshal)
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
			},
		)
	}
}

//------------------------ ECDSA tests ----------------------------

/*	TestECDSA_registerMessageHandler
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

	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	// creating new localTssData and new partyIds
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

	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "ecdsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	tests := []struct {
		name         string
		noAnswer     bool
		senderId     string
		appConfig    func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler)
		localTssData models.TssData
	}{
		{
			name:         "register message with one party in partyId list with no answer false",
			noAnswer:     false,
			senderId:     newPartyId2.Id,
			localTssData: localTssData,
			appConfig: func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler) {
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConnection").Return(conn)

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return(gossipMessage.Signature, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				return app, handler
			},
		},
		{
			name:         "register message with 2 partyId in list with no answer true and senderId equals to localPartyId",
			noAnswer:     true,
			senderId:     newPartyId.Id,
			localTssData: localTssDataWith2PartyIds,
			appConfig: func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				return app, nil
			},
		},
		{
			name:         "register message with 2 partyId in list with no answer true and senderId not equals to localPartyId",
			noAnswer:     true,
			senderId:     newPartyId2.Id,
			localTssData: localTssDataWith2PartyIds,
			appConfig: func(gossipMessage models.GossipMessage) (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				return app, nil
			},
		},
	}

	logger, _ := mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				registerMsg := models.Register{
					Id:        newPartyId.Id,
					Moniker:   newPartyId.Moniker,
					Key:       newPartyId.KeyInt().String(),
					Timestamp: time.Now().Unix() / 60,
					NoAnswer:  tt.noAnswer,
				}
				marshal, err := json.Marshal(registerMsg)
				if err != nil {
					return
				}
				payload := mockUtils.CreatePayload(marshal, tt.senderId, signMsg, registerMessage)
				gossipMessage, err := mockUtils.CreateGossipMessageForECDSA(payload, "", saveData)
				if err != nil {
					t.Errorf(err.Error())
				}
				app, handler := tt.appConfig(gossipMessage)
				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage:  signMsg,
					Signatures:   make(map[int][]byte),
					Logger:       logger,
					Handler:      handler,
				}

				// partyMessageHandler
				err = ecdsaSignOp.RegisterMessageHandler(app, gossipMessage)
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
func TestECDSASign_PartyUpdate(t *testing.T) {

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
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1,
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

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
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
func TestECDSASign_Setup(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	// using mocked function and struct
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)

	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetConfig").Return(
		models.Config{TurnDuration: 60},
	)
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "ecdsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	tests := []struct {
		name         string
		app          _interface.RosenTss
		setupSignMsg models.SetupSign
	}{
		{
			name:         "creating setup message with self setup is empty",
			app:          app,
			setupSignMsg: models.SetupSign{},
		},
		{
			name: "creating setup message with self setup is not empty",
			app:  app,
			setupSignMsg: models.SetupSign{
				Hash: signMsg.Message,
			},
		},
	}

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				ecdsaSignOp := OperationSign{
					LocalTssData:         localTssData,
					SignMessage:          signMsg,
					Logger:               logger,
					Handler:              handler,
					SelfSetupSignMessage: tt.setupSignMsg,
				}
				err := ecdsaSignOp.Setup(tt.app)
				if err != nil {
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
func TestECDSASign_HandleOutMessage(t *testing.T) {
	// creating fake mockUtils.TestUtilsMessage as tss.Message
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	// creating new localTssData
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
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
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger:  logger,
					Handler: handler,
				}
				err := ecdsaSignOp.HandleOutMessage(tt.app, tt.tssMessage)
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
func TestECDSASign_HandleEndMessage(t *testing.T) {
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
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	tests := []struct {
		name          string
		expected      string
		appConfig     func() (_interface.RosenTss, Handler)
		signatureData *common.SignatureData
		tssMessage    tss.Message
	}{
		{
			name:          "handling sign",
			expected:      "there should be an \"message received\" error",
			signatureData: &saveSign,
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On(
					"CallBack",
					mock.AnythingOfType("string"),
					mock.AnythingOfType("models.SignData"),
					mock.AnythingOfType("string"),
				).Return(
					fmt.Errorf("message received"),
				)
				app.On("GetConnection").Return(conn)

				handler := mocks.NewHandler(t)

				return app, handler
			},
		},
		{
			name:       "party message",
			expected:   "there should be an \"message received\" error",
			tssMessage: &message,
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
				app.On("GetConnection").Return(conn)

				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				return app, handler
			},
		},
	}

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				app, handler := tt.appConfig()

				ecdsaSignOp := OperationSign{
					LocalTssData: localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger:  logger,
					Handler: handler,
				}
				outCh := make(chan tss.Message, len(ecdsaSignOp.LocalTssData.PartyIds))
				endCh := make(chan common.SignatureData, len(ecdsaSignOp.LocalTssData.PartyIds))
				switch tt.name {

				case "handling sign":
					endCh <- *tt.signatureData
				case "party message":
					outCh <- tt.tssMessage
				}
				result, err := ecdsaSignOp.GossipMessageHandler(app, outCh, endCh)
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
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewECDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewECDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

	message := models.Payload{
		Message:   "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		MessageId: "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:  "cahj2pgs4eqvn1eo1tp0",
		Name:      "register",
	}
	// using mocked structs and functions
	app := mockedInterface.NewRosenTss(t)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)
	handler := mocks.NewHandler(t)
	handler.On("Sign", mock.Anything).Return([]byte{}, nil)
	handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

	tests := []struct {
		name string
	}{
		{
			name: "create and send new message to p2p",
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
					LocalTssData: localTssData,
					Logger:       logger,
					Handler:      handler,
				}
				err := ecdsaSignOp.NewMessage(app, message, "")
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
	- ecdsaKeygen.LocalPartySaveData
	- tss.Party for ecdsaSign.NewLocalParty
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
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
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

	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "ecdsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}

	msgBytes, _ := hex.DecodeString(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "ecdsa", hex.EncodeToString(messageBytes[:]))

	setupMsg := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     []tss.PartyID{*newPartyId2},
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	marshal, err := json.Marshal(setupMsg)
	if err != nil {
		t.Error(err)
	}
	gossipMessage := models.GossipMessage{
		Message:    utils.Encoder(marshal),
		MessageId:  messageId,
		SenderId:   newPartyId.Id,
		ReceiverId: "",
		Name:       "setup",
	}

	tests := []struct {
		name      string
		appConfig func() (_interface.RosenTss, Handler)
		index     int
	}{
		{
			name: "new setup message, round equals to index",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{TurnDuration: 60},
				)
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				return app, handler
			},
			index: int((time.Now().Unix() / 60) % int64(len(saveData.Ks))),
		},
		{
			name: "new setup message, round not equals to index",
			appConfig: func() (_interface.RosenTss, Handler) {
				app := mockedInterface.NewRosenTss(t)
				app.On("GetConfig").Return(
					models.Config{TurnDuration: 60},
				)
				return app, nil
			},
			index: int((time.Now().Unix()/60)%int64(len(saveData.Ks))) + 1,
		},
	}

	logger, err := mockUtils.InitLog("ecdsa-sign")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app, handler := tt.appConfig()
				ecdsaSignOp := OperationSign{
					LocalTssData: localTssData,
					Logger:       logger,
					SignMessage:  signMsg,
					Handler:      handler,
				}
				gossipMessage.Index = tt.index
				err := ecdsaSignOp.SetupMessageHandler(app, gossipMessage, saveData.Ks)
				if err != nil {
					t.Errorf("SetupMessageHandler error = %v", err)
				}

			},
		)
	}
}

/*	TestECDSASign_SignStarter
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is models.GossipMessage, models.SignMessage, models.SetupSign used as test arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
	- eddsaKeygen.LocalPartySaveData
	- tss.Party for eddsaSign.NewLocalParty
	- network.Publish function
	- rosenTss GetConnection functions
	- handler GetData, Sign functions
*/
func TestECDSASign_SignStarter(t *testing.T) {

	// creating new localTssData and new partyId
	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
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
	signMsg := models.SignMessage{
		Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
		Crypto:      "ecdsa",
		CallBackUrl: "http://localhost:5050/callback/sign",
	}
	msgBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	messageBytes := blake2b.Sum256(signData.Bytes())

	setupSignMsg := models.SetupSign{
		Hash:      utils.Encoder(messageBytes[:]),
		Peers:     []tss.PartyID{*newPartyId2},
		Timestamp: time.Now().Unix() / 60,
		StarterId: newPartyId.Id,
	}

	tests := []struct {
		name         string
		appConfig    func() _interface.RosenTss
		localTssData models.TssData
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
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)

				ecdsaSignOp := OperationSign{
					LocalTssData:     tt.localTssData,
					Logger:           logger,
					SetupSignMessage: setupSignMsg,
					SignMessage:      signMsg,
					Handler:          handler,
				}
				err := ecdsaSignOp.SignStarter(app)
				if err != nil {
					t.Errorf("SignStarter error = %v", err)
				}

			},
		)
	}
}

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
func TestECDSA_Init(t *testing.T) {

	saveData, _, err := mockUtils.LoadECDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadECDSAKeygenFixture error = %v", err)
	}

	// creating fake localTssData
	localTssData, err := mockUtils.CreateNewLocalECDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalECDSATSSData error = %v", err)
	}

	// using mock functions

	tests := []struct {
		name         string
		receiverId   string
		localTssData models.TssData
		appConfig    func() _interface.RosenTss
	}{
		{
			name: "initiate partyId and creating register message",
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
	}

	logger, err := mockUtils.InitLog("ecdsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				handler := mocks.NewHandler(t)
				handler.On("Sign", mock.Anything).Return([]byte{}, nil)
				handler.On("GetData").Return(saveData.Ks, saveData.ShareID)
				handler.On("LoadData", mock.Anything).Return(
					tss.NewPartyID("3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD", "", big.NewInt(10)),
					nil,
				)

				ecdsaSignOp := OperationSign{
					LocalTssData: tt.localTssData,
					SignMessage: models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
					Logger:  logger,
					Handler: handler,
				}

				err := ecdsaSignOp.Init(app, tt.receiverId)
				if err != nil {
					t.Errorf("Init failed: %v", err)
				}

			},
		)
	}
}
