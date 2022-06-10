package storage

import (
	"encoding/json"
	"fmt"
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
	MakefilePath(peerHome string, topicName string, fileFormat string, protocol string)
	WriteData(data interface{}, peerHome string, topicName string, fileFormat string, protocol string) error
	LoadEDDSAKeygen(peerHome string) (eddsaKeygen.LocalPartySaveData, *tss.PartyID, error)
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
func (f *storage) MakefilePath(peerHome string, topicName string, fileFormat string, protocol string) {
	dirName := filepath.Join(peerHome, topicName)
	f.filePath = fmt.Sprintf("%s/%s/"+fileFormat, dirName, protocol)
}

// WriteData writing given data to file in given path
func (f *storage) WriteData(data interface{}, peerHome string, topicName string, fileFormat string, protocol string) error {

	f.MakefilePath(peerHome, topicName, fileFormat, protocol)

	fi, err := os.Stat(f.filePath)
	if !(err == nil && !fi.IsDir()) {
		fd, err := os.OpenFile(f.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		defer fd.Close()

		if err != nil {
			return fmt.Errorf("unable to open File %s for writing, err:{%v}", f.filePath, err)
		}
		bz, err := json.MarshalIndent(&data, "", "    ")
		if err != nil {
			return fmt.Errorf("unable to marshal save data for File %s, err:{%v}", f.filePath, err)
		}
		_, err = fd.Write(bz)
		if err != nil {
			return fmt.Errorf("unable to write to File %s", f.filePath)
		}
		models.Logger.Infof("Saved a File: %s", f.filePath)
		return nil
	}
	return fmt.Errorf("file stat error: %v", err)

}

// LoadEDDSAKeygen Loads the EDDSA keygen data from the file
func (f *storage) LoadEDDSAKeygen(peerHome string) (eddsaKeygen.LocalPartySaveData, *tss.PartyID, error) {
	// locating file
	var keygenFile string

	rootFolder := filepath.Join(peerHome, "eddsa")
	files, err := ioutil.ReadDir(rootFolder)
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
	filePath := filepath.Join(rootFolder, keygenFile)
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
	models.Logger.Infof("key: %+v", key)

	var parties tss.UnSortedPartyIDs
	parties = append(parties, partyID)
	sortedPIDs := tss.SortPartyIDs(parties)
	return key, sortedPIDs[0], nil
}
