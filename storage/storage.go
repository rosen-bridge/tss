package storage

import (
	"encoding/json"
	"fmt"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"io/ioutil"
	"os"
	"path/filepath"
	"rosen-bridge/tss/models"
	"strings"
)

type Storage interface {
	MakefilePath(peerHome string, protocol string)
	WriteData(data interface{}, peerHome string, fileFormat string, protocol string) error
	LoadEDDSAKeygen(peerHome string) (eddsaKeygen.LocalPartySaveData, *tss.PartyID, error)
	LoadECDSAKeygen(peerHome string) (ecdsaKeygen.LocalPartySaveData, *tss.PartyID, error)
	LoadPrivate(peerHome string, crypto string) string
}

type storage struct {
	filePath string
}

// NewStorage Constructor of a storage struct
func NewStorage() Storage {
	return &storage{
		filePath: "",
	}
}

// MakefilePath Constructor of a storage struct
func (f *storage) MakefilePath(peerHome string, protocol string) {
	f.filePath = fmt.Sprintf("%s/%s", peerHome, protocol)
}

// WriteData writing given data to file in given path
func (f *storage) WriteData(data interface{}, peerHome string, fileFormat string, protocol string) error {

	models.Logger.Info("write data called")

	f.MakefilePath(peerHome, protocol)
	err := os.MkdirAll(f.filePath, os.ModePerm)
	if err != nil {
		return err
	}

	path := filepath.Join(f.filePath, fileFormat)

	models.Logger.Infof("path: %s", path)
	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer func(fd *os.File) {
		err := fd.Close()
		if err != nil {
			models.Logger.Errorf("unable to Close File %s, err:{%v}", path, err)
		}
	}(fd)

	if err != nil {
		return fmt.Errorf("unable to open File %s for writing, err:{%v}", path, err)
	}
	bz, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return fmt.Errorf("unable to marshal save data for File %s, err:{%v}", path, err)
	}
	_, err = fd.Write(bz)
	if err != nil {
		return fmt.Errorf("unable to write to File %s", path)
	}
	models.Logger.Infof("Saved a File: %s", path)
	return nil
}

// LoadEDDSAKeygen Loads the EDDSA keygen data from the file
func (f *storage) LoadEDDSAKeygen(peerHome string) (eddsaKeygen.LocalPartySaveData, *tss.PartyID, error) {
	// locating file
	var keygenFile string

	f.MakefilePath(peerHome, "eddsa")
	files, err := ioutil.ReadDir(f.filePath)
	if err != nil {
		return eddsaKeygen.LocalPartySaveData{}, nil, err
	}
	if len(files) == 0 {
		return eddsaKeygen.LocalPartySaveData{}, nil, errors.New("no keygen data found")
	}
	for _, File := range files {
		if strings.Contains(File.Name(), "keygen") {
			keygenFile = File.Name()
		}
	}
	filePath := filepath.Join(f.filePath, keygenFile)
	models.Logger.Infof("File: %v", filePath)

	// reading file
	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		return eddsaKeygen.LocalPartySaveData{}, nil, errors.Wrapf(err,
			"could not open the File for party in the expected location: %s. run keygen first.", filePath)
	}
	var key eddsaKeygen.LocalPartySaveData
	if err = json.Unmarshal(bz, &key); err != nil {
		return eddsaKeygen.LocalPartySaveData{}, nil, errors.Wrapf(err,
			"could not unmarshal data for party located at: %s", filePath)
	}

	//creating data from file
	for _, kbxj := range key.BigXj {
		kbxj.SetCurve(tss.Edwards())
	}
	key.EDDSAPub.SetCurve(tss.Edwards())
	id := xid.New()
	pMoniker := fmt.Sprintf("tssPeer/%s", id.String())
	partyID := tss.NewPartyID(id.String(), pMoniker, key.ShareID)

	var parties tss.UnSortedPartyIDs
	parties = append(parties, partyID)
	sortedPIDs := tss.SortPartyIDs(parties)
	return key, sortedPIDs[0], nil
}

// LoadECDSAKeygen Loads the ECDSA keygen data from the file
func (f *storage) LoadECDSAKeygen(peerHome string) (ecdsaKeygen.LocalPartySaveData, *tss.PartyID, error) {
	// locating file
	var keygenFile string

	f.MakefilePath(peerHome, "ecdsa")
	files, err := ioutil.ReadDir(f.filePath)
	if err != nil {
		return ecdsaKeygen.LocalPartySaveData{}, nil, err
	}
	if len(files) == 0 {
		return ecdsaKeygen.LocalPartySaveData{}, nil, errors.New("no keygen data found")
	}
	for _, File := range files {
		if strings.Contains(File.Name(), "keygen") {
			keygenFile = File.Name()
		}
	}
	filePath := filepath.Join(f.filePath, keygenFile)
	models.Logger.Infof("File: %v", filePath)

	// reading file
	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ecdsaKeygen.LocalPartySaveData{}, nil, errors.Wrapf(err,
			"could not open the File for party in the expected location: %s. run keygen first.", filePath)
	}
	var key ecdsaKeygen.LocalPartySaveData
	if err = json.Unmarshal(bz, &key); err != nil {
		return ecdsaKeygen.LocalPartySaveData{}, nil, errors.Wrapf(err,
			"could not unmarshal data for party located at: %s", filePath)
	}

	//creating data from file
	for _, kbxj := range key.BigXj {
		kbxj.SetCurve(tss.S256())
	}
	key.ECDSAPub.SetCurve(tss.S256())
	id := xid.New()
	pMoniker := fmt.Sprintf("tssPeer/%s", id.String())
	partyID := tss.NewPartyID(id.String(), pMoniker, key.ShareID)

	var parties tss.UnSortedPartyIDs
	parties = append(parties, partyID)
	sortedPIDs := tss.SortPartyIDs(parties)
	return key, sortedPIDs[0], nil
}

// LoadPrivate Loads the private data from the file
func (f *storage) LoadPrivate(peerHome string, crypto string) string {
	// locating file
	var privateFile string

	f.MakefilePath(peerHome, crypto)
	files, err := ioutil.ReadDir(f.filePath)
	if err != nil {
		models.Logger.Error(err)
		return ""
	}
	if len(files) == 0 {
		models.Logger.Error(fmt.Errorf("no private data found"))
		return ""
	}
	for _, File := range files {
		if strings.Contains(File.Name(), "private") {
			privateFile = File.Name()
		}
	}

	if privateFile == "" {
		return ""
	}
	filePath := filepath.Join(f.filePath, privateFile)
	models.Logger.Infof("File: %v", filePath)

	// reading file
	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		models.Logger.Error(fmt.Errorf("could not open the File for party in the expected location: %s. import private first. error: %v", filePath, err))
		return ""
	}
	var key models.Private
	if err = json.Unmarshal(bz, &key); err != nil {
		models.Logger.Error(fmt.Errorf("could not unmarshal private data located at: %s, err: %v", filePath, err))
		return ""
	}

	//creating data from file
	return key.Private
}
