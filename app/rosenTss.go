package app

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/gommon/log"
	"golang.org/x/crypto/blake2b"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
	"strings"
)

// rosenTss type can hold some data and methods
type rosenTss struct {
	ChannelMap map[string]chan models.Message
	metaData   models.MetaData
	storage    storage.Storage
	connection network.Connection
	Private    models.Private
	peerHome   string
}

// NewRosenTss Constructor of a app app
func NewRosenTss(connection network.Connection, storage storage.Storage, homeAddress string) _interface.RosenTss {
	return &rosenTss{
		ChannelMap: make(map[string]chan models.Message),
		metaData:   models.MetaData{},
		storage:    storage,
		connection: connection,
		Private:    models.Private{},
		peerHome:   homeAddress,
	}
}

// StartNewSign starts sign scenario for app based on given protocol.
func (r *rosenTss) StartNewSign(signMessage models.SignMessage) error {
	models.Logger.Info("Starting New Sign process")
	err := r.SetMetaData()
	if err != nil {
		return err
	}

	messageCh := make(chan models.Message, 100)

	msgBytes, _ := hex.DecodeString(signMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	signDtaHash := hex.EncodeToString(signDataBytes[:])
	r.ChannelMap[signDtaHash] = messageCh

	// read loop function
	if signMessage.Crypto == "ecdsa" {
		// TODO: implement this

	} else if signMessage.Crypto == "eddsa" {
		EDDSAOperation := sign.NewSignEDDSAOperation()
		err := EDDSAOperation.Init(r)
		if err != nil {
			return err
		}
		go func() {
			err := EDDSAOperation.Loop(r, messageCh, signData)
			if err != nil {
				models.Logger.Error(err)
			}
		}()
	}
	return nil
}

func (r *rosenTss) GetStorage() storage.Storage {
	return r.storage
}
func (r *rosenTss) GetConnection() network.Connection {
	return r.connection
}

func (r *rosenTss) SetPeerHome(homeAddress string) error {
	models.Logger.Info("setting up home directory")

	if homeAddress[0:1] == "." {
		absHomeAddress, err := filepath.Abs(homeAddress)
		if err != nil {
			return err
		}
		r.peerHome = absHomeAddress
	} else if homeAddress[0:1] == "~" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		absHomeAddress := filepath.Join(userHome, homeAddress[1:])
		r.peerHome = absHomeAddress
	} else {
		r.peerHome = homeAddress
	}

	if err := os.MkdirAll(r.peerHome, os.ModePerm); err != nil {
		models.Logger.Error(err)
		return err
	}
	return nil
}
func (r *rosenTss) GetPeerHome() string {
	return r.peerHome
}

func (r *rosenTss) SetMetaData() error {
	// locating file
	var configFile string
	rootFolder := filepath.Join(r.peerHome, "eddsa")
	files, err := ioutil.ReadDir(rootFolder)
	if err != nil {
		log.Error(err)
	}
	if len(files) == 0 {
		return errors.New("no keygen data found")
	}
	for _, File := range files {
		if strings.Contains(File.Name(), "config") {
			configFile = File.Name()
		}
	}
	filePath := filepath.Join(rootFolder, configFile)
	log.Infof("File: %v", filePath)

	// reading file
	bz, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf(
			"{%#v}\n could not open the File for party in the expected location: %s. run keygen first.", err, filePath)
	}
	var meta models.MetaData
	if err = json.Unmarshal(bz, &meta); err != nil {
		return fmt.Errorf(
			"{%#v}\n could not unmarshal data for party located at: %s", err, filePath)
	}

	r.metaData = meta
	return nil
}
func (r *rosenTss) GetMetaData() models.MetaData {
	return r.metaData
}

// NewMessage cretes messages for app rounds
func (r *rosenTss) NewMessage(id string, message string, messageId string, name string) []byte {

	m := models.GossipMessage{
		Message:   message,
		MessageId: messageId,
		SenderID:  id,
		Name:      name,
	}
	msgBytes, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	// TODO: create message struct
	return msgBytes
}
