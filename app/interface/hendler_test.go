package _interface

import (
	"encoding/hex"
	"encoding/json"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"rosen-bridge/tss/mocks"
	"rosen-bridge/tss/models"
	"strings"
	"testing"
)

/*	TestHandler_PartyMessageHandler
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is a models.Message used as a function argument.
	Dependencies:
	-
*/
func TestHandler_PartyMessageHandler(t *testing.T) {

	message := mocks.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	tests := []struct {
		name     string
		partyMsg tss.Message
	}{
		{
			name:     "creating Keygen message, PartyMessage model should create successfully",
			partyMsg: &message,
		},
	}

	operation := OperationHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// partyMessageHandler
			partyMessageBytes, err := operation.PartyMessageHandler(tt.partyMsg)
			if err != nil {
				t.Error(err)
			}
			partyMessage := models.PartyMessage{}
			decodeString, err := hex.DecodeString(partyMessageBytes)
			if err != nil {
				t.Error(err)
			}

			err = json.Unmarshal(decodeString, &partyMessage)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, hex.EncodeToString(partyMessage.Message), message.Data)
		})
	}

}

/*	TestHandler_SharedPartyUpdater
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are a models.PartyMessage and tss.Party used as function arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.Party for eddsaKeygen
*/
func TestHandler_SharedPartyUpdater(t *testing.T) {

	// creating fake localTssData
	localTssData, err := mocks.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Error(err)
	}

	newPartyId, err := mocks.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	// creating fake tss.Party
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	localTssData.Party = eddsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	// fake models.PartyMessage

	tests := []struct {
		name    string
		message models.PartyMessage
		party   tss.Party
	}{
		{
			name: "updating shared party, party = getFrom",
			message: models.PartyMessage{
				To:                      []*tss.PartyID{localTssData.PartyID},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			party: localTssData.Party,
		},
		{
			name: "updating shared party, party != getFrom",
			message: models.PartyMessage{
				To:                      []*tss.PartyID{localTssData.PartyID},
				GetFrom:                 newPartyId,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			party: localTssData.Party,
		},
	}

	operation := OperationHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = operation.SharedPartyUpdater(tt.party, tt.message)
			if err != nil && !strings.Contains(err.Error(), "invalid wire-format data") {
				t.Error(err)
			}
		})
	}
}
