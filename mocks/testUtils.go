package mocks

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/labstack/gommon/log"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"runtime"
	"strings"
)

// TestUtilsMessage internal struct for mocking tss.Message
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

// GenerateEDDSAKey generates private and public for edward curve
func GenerateEDDSAKey() ([]byte, *big.Int, *big.Int, error) {
	return edwards.GenerateKey(rand.Reader)
}

// CreateNewEDDSAPartyId creates a new partyId with edward key
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

// CreateNewLocalEDDSATSSData creates a new partyId with edward key and setting it in localTssData
func CreateNewLocalEDDSATSSData() (models.TssData, error) {
	newPartyId, err := CreateNewEDDSAPartyId()
	if err != nil {
		return models.TssData{}, err
	}
	localTssData := models.TssData{
		PartyID: newPartyId,
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	return localTssData, nil
}

// LoadEDDSAKeygenFixture reads eddsa keygen fixer file by given index
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

// Contains check if item exist in the list or not
func Contains(item string, itemList []string) bool {
	for _, element := range itemList {
		if strings.Contains(item, element) {
			return true
		}
	}
	return false
}

// GenerateECDSAKey generates private and public for edward curve
func GenerateECDSAKey() ([]byte, *big.Int, *big.Int, error) {
	return elliptic.GenerateKey(elliptic.P256(), rand.Reader)
}

// CreateNewEDDSAPartyId creates a new partyId with secp256k1 key
func CreateNewECDSAPartyId() (*tss.PartyID, error) {
	private, _, _, err := GenerateECDSAKey()
	if err != nil {
		private = nil
		return nil, err
	}
	key := new(big.Int).SetBytes(private)
	id := xid.New()
	partyId := tss.NewPartyID(id.String(), "topic"+"/tssPeer", key)
	return partyId, nil
}

// CreateNewLocalECDSATSSData creates a new partyId with secp256k1 key and setting it in localTssData
func CreateNewLocalECDSATSSData() (models.TssData, error) {
	newPartyId, err := CreateNewECDSAPartyId()
	if err != nil {
		return models.TssData{}, err
	}
	localTssData := models.TssData{
		PartyID: newPartyId,
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	return localTssData, nil
}

// LoadECDSAKeygenFixture reads eddsa keygen fixer file by given index
func LoadECDSAKeygenFixture(index int) (ecdsaKeygen.LocalPartySaveData, *tss.PartyID, error) {
	testFixtureDirFormat := "%s/../mocks/_ecdsa_keygen_fixtures"

	_, callerFileName, _, _ := runtime.Caller(0)
	srcDirName := filepath.Dir(callerFileName)
	rootFolder := fmt.Sprintf(testFixtureDirFormat, srcDirName)
	files, err := ioutil.ReadDir(rootFolder)

	keygenFile := files[index].Name()

	if err != nil {
		log.Error(err)
	}
	if len(files) == 0 {
		return ecdsaKeygen.LocalPartySaveData{}, nil, errors.New("no keygen data found")
	}

	filePath := filepath.Join(rootFolder, keygenFile)
	log.Infof("File: %v", filePath)
	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ecdsaKeygen.LocalPartySaveData{}, nil, fmt.Errorf(
			"could not open the File for party in the expected location: %s. run keygen first.\nerror:{%s}",
			filePath, err.Error())
	}
	var key ecdsaKeygen.LocalPartySaveData
	if err = json.Unmarshal(bz, &key); err != nil {
		return ecdsaKeygen.LocalPartySaveData{}, nil, fmt.Errorf(
			"could not unmarshal data for party located at: %s\nerror: {%s}", filePath, err.Error())
	}
	for _, kbxj := range key.BigXj {
		kbxj.SetCurve(tss.S256())
	}
	key.ECDSAPub.SetCurve(tss.S256())
	id := xid.New()
	pMoniker := fmt.Sprintf("%s", id.String())
	partyID := tss.NewPartyID(pMoniker, "tss/tssPeer", key.ShareID)
	var parties tss.UnSortedPartyIDs
	parties = append(parties, partyID)
	sortedPIDs := tss.SortPartyIDs(parties)
	return key, sortedPIDs[0], nil
}

func InitLog(name string) (*zap.SugaredLogger, error) {
	err := logger.Init("/tmp/tss.log", "debug", 1, 5, 2, false)
	if err != nil {
		return nil, err
	}
	return logger.NewSugar(name), nil
}

// GetPKFromECDSAPub returns the public key from an ECDSA public key
func GetPKFromECDSAPub(x *big.Int, y *big.Int) []byte {
	return elliptic.MarshalCompressed(elliptic.P256(), x, y)
}

// GetPKFromEDDSAPub returns the public key Serialized from an EDDSA public key.
func GetPKFromEDDSAPub(x *big.Int, y *big.Int) []byte {
	return edwards.NewPublicKey(x, y).Serialize()
}

func GetEcdsaPK() (string, error) {
	saveData, _, err := LoadECDSAKeygenFixture(0)
	if err != nil {
		return "", err
	}
	pkX, pkY := saveData.ECDSAPub.X(), saveData.ECDSAPub.Y()
	pk := ecdsa.PublicKey{
		Curve: tss.S256(),
		X:     pkX,
		Y:     pkY,
	}

	public := GetPKFromECDSAPub(pk.X, pk.Y)
	hexPk := hex.EncodeToString(public)
	return hexPk, nil
}

func GetEddsaPK() (string, error) {
	saveData, _, err := LoadEDDSAKeygenFixture(0)
	if err != nil {
		return "", err
	}
	pkX, pkY := saveData.EDDSAPub.X(), saveData.EDDSAPub.Y()
	pk := edwards.PublicKey{
		Curve: tss.Edwards(),
		X:     pkX,
		Y:     pkY,
	}

	public := GetPKFromEDDSAPub(pk.X, pk.Y)
	hexPk := hex.EncodeToString(public)
	return hexPk, nil
}
