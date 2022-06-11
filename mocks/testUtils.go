package mocks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/labstack/gommon/log"
	"github.com/rs/xid"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"rosen-bridge/tss/models"
	"runtime"
	"strings"
)

type TestUtilsMessage struct {
	Broadcast    bool
	From         string
	To           string
	Data         string
	oldCommittee bool
	newCommittee bool
}

func (tu *TestUtilsMessage) Type() string {
	return "implement me"
}

func (tu *TestUtilsMessage) GetTo() []*tss.PartyID {
	newPartyId, _ := CreateNewEDDSAPartyId()
	return []*tss.PartyID{newPartyId}
}

func (tu *TestUtilsMessage) GetFrom() *tss.PartyID {
	newPartyId, _ := CreateNewEDDSAPartyId()
	return newPartyId
}

func (tu *TestUtilsMessage) IsToOldCommittee() bool {
	return tu.oldCommittee
}

func (tu *TestUtilsMessage) IsToOldAndNewCommittees() bool {
	if tu.oldCommittee && tu.newCommittee {
		return true
	}
	return false
}

func (tu *TestUtilsMessage) WireBytes() ([]byte, *tss.MessageRouting, error) {
	data, _ := hex.DecodeString(tu.Data)
	return data, nil, nil
}

func (tu *TestUtilsMessage) WireMsg() *tss.MessageWrapper {
	return nil
}

func (tu *TestUtilsMessage) String() string {
	return ""
}

func (tu *TestUtilsMessage) IsBroadcast() bool {
	return tu.Broadcast
}

func GenerateEDDSAKey() ([]byte, *big.Int, *big.Int, error) {
	return edwards.GenerateKey(rand.Reader)
}

func CreateNewEDDSAPartyId() (*tss.PartyID, error) {
	private, _, _, err := GenerateEDDSAKey()
	if err != nil {
		private = nil
		return nil, err
	}
	key := new(big.Int).SetBytes(private)
	id := xid.New()
	partyId := tss.NewPartyID(id.String(), "topic"+"/tssPeer", key)
	return partyId, nil
}

func CreateNewLocalEDDSATSSData() (models.TssData, error) {
	newParty, err := CreateNewEDDSAPartyId()
	if err != nil {
		return models.TssData{}, err
	}
	localTssData := models.TssData{
		PartyID: newParty,
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newParty))

	return localTssData, nil
}

func LoadEDDSAKeygenFixture(index int) (eddsaKeygen.LocalPartySaveData, *tss.PartyID, error) {
	testFixtureDirFormat := "%s/../mocks/_eddsa_keygen_fixtures"

	_, callerFileName, _, _ := runtime.Caller(0)
	srcDirName := filepath.Dir(callerFileName)
	rootFolder := fmt.Sprintf(testFixtureDirFormat, srcDirName)
	files, err := ioutil.ReadDir(rootFolder)

	keygenFile := files[index].Name()

	if err != nil {
		log.Error(err)
	}
	if len(files) == 0 {
		return eddsaKeygen.LocalPartySaveData{}, nil, errors.New("no keygen data found")
	}

	filePath := filepath.Join(rootFolder, keygenFile)
	log.Infof("File: %v", filePath)
	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		return eddsaKeygen.LocalPartySaveData{}, nil, fmt.Errorf(
			"could not open the File for party in the expected location: %s. run keygen first.\nerror:{%s}",
			filePath, err.Error())
	}
	var key eddsaKeygen.LocalPartySaveData
	if err = json.Unmarshal(bz, &key); err != nil {
		return eddsaKeygen.LocalPartySaveData{}, nil, fmt.Errorf(
			"could not unmarshal data for party located at: %s\nerror: {%s}", filePath, err.Error())
	}
	for _, kbxj := range key.BigXj {
		kbxj.SetCurve(tss.Edwards())
	}
	key.EDDSAPub.SetCurve(tss.Edwards())
	id := xid.New()
	pMoniker := fmt.Sprintf("%s", id.String())
	partyID := tss.NewPartyID(pMoniker, "tss/tssPeer", key.ShareID)
	var parties tss.UnSortedPartyIDs
	parties = append(parties, partyID)
	sortedPIDs := tss.SortPartyIDs(parties)
	return key, sortedPIDs[0], nil
}

func Contains(item string, itemList []string) bool {
	for _, element := range itemList {
		if strings.Contains(item, element) {
			return true
		}
	}
	return false
}
