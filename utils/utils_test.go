package utils

import (
	"github.com/binance-chain/tss-lib/tss"
	"rosen-bridge/tss/mocks"
	"testing"
)

/*	TestUtils_IsPartyExist
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are a tss.SortedPartyIDs and tss.PartyId used as function arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
*/
func TestUtils_IsPartyExist(t *testing.T) {

	// creating fake localTssData
	localTssData, err := mocks.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Error(err)
	}

	// creating fake partyId
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
			name:     "partyId exist in the partyId list, expected true",
			partyIds: localTssData.PartyIds,
			partyId:  localTssData.PartyID,
			expected: true,
		},
		{
			name:     "partyId not exist in the partyId list, expected false",
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
