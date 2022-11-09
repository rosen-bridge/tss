package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	"rosen-bridge/tss/mocks"
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
		t.Run(
			tt.name, func(t *testing.T) {
				result := IsPartyExist(tt.partyId, tt.partyIds)
				if result != tt.expected {
					t.Error(err)
				}
			},
		)
	}
}

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
func TestUtils_GetErgoAddressFromPK(t *testing.T) {
	// creating fake localTssData
	_, x, y, err := GenerateECDSAKey()
	if err != nil {
		t.Error(err)
	}
	compressedPk := GetPKFromECDSAPub(x, y)

	tests := []struct {
		name    string
		pk      []byte
		testNet bool
	}{
		{
			name:    "creating mainNet ergo address from compressedPk",
			pk:      compressedPk,
			testNet: false,
		},
		{
			name:    "creating testNet ergo address from compressedPk",
			pk:      compressedPk,
			testNet: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := GetErgoAddressFromPK(tt.pk, tt.testNet)
				t.Log(result)
				t.Log(len(result))
				if tt.testNet {
					assert.Equal(t, result[:1], "3")
					assert.Equal(t, len(result), 52)
				} else {
					assert.Equal(t, result[:1], "9")
					assert.Equal(t, len(result), 51)
				}
			},
		)
	}
}

/*	TestUtils_GetAbsoluteAddress
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are a tss.SortedPartyIDs and tss.PartyId used as function arguments.
	Dependencies:
	- localTssData models.TssData
	- tss.PartyId
*/
func TestUtils_GetAbsoluteAddress(t *testing.T) {
	// get user home dir
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Error(err)
	}
	// creating absolute path from relative path
	absHomeAddress, err := filepath.Abs("./.rosenTss")
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name        string
		homeAddress string
		expected    string
		wantErr     bool
	}{
		{
			name:        "relative home address, should be equal to expected",
			homeAddress: "./.rosenTss",
			expected:    absHomeAddress,
			wantErr:     false,
		},
		{
			name:        "user home address, should be equal to expected",
			homeAddress: "~/tmp/.rosenTss",
			expected:    fmt.Sprintf("%s/tmp/.rosenTss", userHome),
			wantErr:     false,
		},
		{
			name:        "complete home address, should be equal to expected",
			homeAddress: "/tmp/.rosenTss",
			expected:    "/tmp/.rosenTss",
			wantErr:     false,
		},
		{
			name:        "complete home address, should be equal to expected",
			homeAddress: "!tmp/.rosenTss",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result, err := GetAbsoluteAddress(tt.homeAddress)
				if err != nil && !tt.wantErr {
					t.Fatal(err)
				}
				assert.Equal(t, result, tt.expected)
			},
		)
	}
}
