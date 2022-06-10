package _interface

import (
	"encoding/hex"
	"encoding/json"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"rosen-bridge/tss/mocks"
	"rosen-bridge/tss/models"
	"testing"
)

func TestHandler_PartyMessageHandler(t *testing.T) {
	message := mocks.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	tests := []struct {
		name     string
		partyMsg tss.Message
	}{
		{name: "creating Keygen message", partyMsg: &message},
	}
	operation := OperationHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// partyMessageHandler
			partyMessageBytes, err := operation.PartyMessageHandler(tt.partyMsg)
			if err != nil {
				t.Fatal(err)
			}
			partyMessage := models.PartyMessage{}
			decodeString, err := hex.DecodeString(partyMessageBytes)
			if err != nil {
				t.Fatal(err)
			}

			err = json.Unmarshal(decodeString, &partyMessage)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, hex.EncodeToString(partyMessage.Message), message.Data)
		})
	}

}

func TestHandler_SharedPartyUpdater(t *testing.T) {
	localTssData, err := mocks.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Fatal(err)
	}
	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	localTssData.Party = eddsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	message := models.PartyMessage{
		To:                      []*tss.PartyID{localTssData.PartyID},
		GetFrom:                 localTssData.PartyID,
		IsBroadcast:             true,
		Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
		IsToOldCommittee:        true,
		IsToOldAndNewCommittees: false,
	}

	tests := []struct {
		name    string
		message models.PartyMessage
		party   tss.Party
	}{
		{
			name:    "updating shared party",
			message: message,
			party:   localTssData.Party,
		},
	}

	operation := OperationHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := operation.SharedPartyUpdater(tt.party, tt.message)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHandler_IsExist(t *testing.T) {
	localTssData, err := mocks.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Fatal(err)
	}

	newPartyId, err := mocks.CreateNewEDDSAPartyId()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		partyIds tss.SortedPartyIDs
		partyId  *tss.PartyID
		expected bool
	}{
		{
			name:     "exist",
			partyIds: localTssData.PartyIds,
			partyId:  localTssData.PartyID,
			expected: true,
		},
		{
			name:     "not exist",
			partyIds: localTssData.PartyIds,
			partyId:  newPartyId,
			expected: false,
		},
	}

	operation := OperationHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := operation.IsExist(tt.partyId, tt.partyIds)
			if result != tt.expected {
				t.Fatal(err)
			}
		})
	}
}
