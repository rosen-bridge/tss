package utils

import (
	"github.com/binance-chain/tss-lib/tss"
	"rosen-bridge/tss/mocks"
	"testing"
)

func TestHandler_IsExist(t *testing.T) {
	localTssData, err := mocks.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Error(err)
	}

	newPartyId, err := mocks.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPartyExist(tt.partyId, tt.partyIds)
			if result != tt.expected {
				t.Error(err)
			}
		})
	}
}
