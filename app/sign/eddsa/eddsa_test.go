package eddsa

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	"rosen-bridge/tss/app/sign"
	mockUtils "rosen-bridge/tss/mocks"
	mockedApp "rosen-bridge/tss/mocks/app/interface"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
)

/*	TestEDDSA_GetClassName
	TestCases:
	testing GetClassName,
	there must be no error and result must be true
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
		t.Run(
			tt.name, func(t *testing.T) {
				eddsaSignOp := operationEDDSASign{
					OperationSign: sign.OperationSign{
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

			},
		)
	}
}

/*	TestEDDSA_Sign
	TestCases:
	testing SignStarterThread, there is 1 testcase.
	creating a signature for message based on given save data,
	there must be no error in signature process
*/
func TestEDDSA_Sign(t *testing.T) {
	// creating fake sign data

	m, _ := utils.Decoder("951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70")

	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	tests := []struct {
		name    string
		message []byte
	}{
		{
			name:    "signing a message",
			message: m,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				testEddsaHandler := handler{
					savedData: saveData,
				}
				signature, err := testEddsaHandler.Sign(tt.message)
				if err != nil {
					t.Errorf("signMessage error = %v", err)
				}
				if len(signature) != 64 {
					t.Errorf("wrong signature = %v, len(signature): %d", signature, len(signature))
				}

			},
		)
	}
}

/*	TestEDDSA_Verify
	TestCases:
	testing SignStarterThread, there is 1 testcase.
	creating signature from message and then the verification must be true.
*/
func TestEDDSA_Verify(t *testing.T) {
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
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID),
	)
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId),
	)

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
		Message:   utils.Encoder(marshal),
		MessageId: "eddsaccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:  newPartyId.Id,
		Name:      "register",
	}
	marshal, err = json.Marshal(payload)
	if err != nil {
		t.Errorf("error = %v", err)
	}

	testEddsaHandler := handler{
		savedData: saveData,
	}

	testEddsaHandler2 := handler{
		savedData: saveData2,
	}

	tests := []struct {
		name    string
		message []byte
	}{
		{
			name:    "verifying a signature on payload from other peer",
			message: marshal,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				signature, err := testEddsaHandler2.Sign(tt.message)

				index, err := testEddsaHandler2.savedData.OriginalIndex()
				if err != nil {
					t.Error(err)
				}
				err = testEddsaHandler.Verify(tt.message, signature, index)
				if err != nil {
					t.Errorf("verify error = %v", err)
				}
			},
		)
	}
}

/*	TestEDDSA_GetData
	TestCases:
	testing SignStarterThread, there is 1 testcase.
	the result must be true
*/
func TestEDDSA_GetData(t *testing.T) {

	saveData, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "get save data from object",
			expected: "data be correct",
		},
	}

	testEddsaHandler := handler{
		savedData: saveData,
	}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				keyList, shareId := testEddsaHandler.GetData()
				for i, key := range keyList {
					if key.Cmp(testEddsaHandler.savedData.Ks[i]) != 0 {
						t.Errorf("wrong key")
					}
				}
				if shareId.Cmp(testEddsaHandler.savedData.ShareID) != 0 {
					t.Errorf("wrong sharedId")
				}

			},
		)
	}
}

/*	TestEDDSA_LoadData
	TestCases:
	testing SignStarterThread, there is 1 testcase.
	there must be no error and the result must be true
*/
func TestEDDSA_LoadData(t *testing.T) {

	saveData, pId, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	app := mockedApp.NewRosenTss(t)
	app.On("GetP2pId").Return("3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD")
	app.On("GetPeerHome").Return("/tmp/.rosenTss")
	storage := mockedStorage.NewStorage(t)
	storage.On("LoadEDDSAKeygen", mock.AnythingOfType("string")).Return(saveData, pId, nil)
	app.On("GetStorage").Return(storage)

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "get save data from object",
			expected: "data be correct",
		},
	}

	testEddsaHandler := handler{}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				pID, err := testEddsaHandler.LoadData(app)
				if err != nil {
					t.Error(err)
				}
				if pID.Id != "3WwwAUB59kz4K5XPeNb5Gog2paQ6hHvAXtFVNvM9zWJqGd6QHZKD" {
					t.Error("wrong pid")
				}
				if pID.KeyInt().Cmp(pId.KeyInt()) != 0 {
					t.Error("wrong pid key")
				}
				if len(testEddsaHandler.savedData.Ks) == 0 {
					t.Error("empty save data")
				}
			},
		)
	}
}

/*	TestEDDSA_StartParty
	TestCases:
	testing SignStarterThread, there is 1 testcase.
	based on local data the process should create and start party
	there must be no error
*/
func TestEDDSA_StartParty(t *testing.T) {
	// pre-test part, faking data and using mocks

	// reading eddsaKeygen.LocalPartySaveData from fixtures
	saveData, Id1, err := mockUtils.LoadEDDSAKeygenFixture(1)
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
	signDataBytes, _ := utils.Decoder(signMsg.Message)
	signData := new(big.Int).SetBytes(signDataBytes)

	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "create party with given data",
			expected: "there must be no error",
		},
	}

	testEddsaHandler := handler{
		savedData: saveData,
	}

	logging, _ = mockUtils.InitLog("eddsa-sign")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

				err = testEddsaHandler.StartParty(&localTssData, localTssData.PartyIds, 2, signData, outCh, endCh)
				if err != nil {
					t.Error(err)
				}
				if !localTssData.Party.Running() {
					t.Errorf("party is not running")
				}
			},
		)
	}
}
