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
	log.Printf("Starting New Sign process")
	err := r.SetMetaData()
	if err != nil {
		return err
	}

	msgBytes, _ := hex.DecodeString(signMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	signDtaHash := hex.EncodeToString(signDataBytes[:])
	log.Printf("signDtaHash: %v", signDtaHash)

	if _, ok := r.ChannelMap[signDtaHash]; !ok {
		messageCh := make(chan models.Message, 100)
		r.ChannelMap[signDtaHash] = messageCh
		models.Logger.Infof("creating new channel in StartNewSign: %v", signDtaHash)

	}

	// read loop function
	if signMessage.Crypto == "ecdsa" {
		// TODO: implement this

	} else if signMessage.Crypto == "eddsa" {
		EDDSAOperation := sign.NewSignEDDSAOperation(signMessage)
		err := EDDSAOperation.Init(r, "")
		if err != nil {
			return err
		}
		go func() {
			models.Logger.Info("calling loop")
			err := EDDSAOperation.Loop(r, r.ChannelMap[signDtaHash])
			if err != nil {
				models.Logger.Error(err)
				//TODO: handle error
			}
			models.Logger.Info("end of  loop")
		}()
	}
	return nil
}

func (r *rosenTss) MessageHandler(message models.Message) {

	models.Logger.Infof("new message: %v", message)
	if _, ok := r.ChannelMap[message.Message.MessageId]; !ok {
		models.Logger.Infof("creating new channel in MessageHandler: %v", message.Message.MessageId)
		messageCh := make(chan models.Message, 100)
		r.ChannelMap[message.Message.MessageId] = messageCh
	}
	r.ChannelMap[message.Message.MessageId] <- message
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
		return errors.New("no config data found")
	}
	for _, File := range files {
		if strings.Contains(File.Name(), "config") {
			configFile = File.Name()
		}
	}
	filePath := filepath.Join(rootFolder, configFile)
	models.Logger.Infof("File: %v", filePath)

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

// NewMessage creates messages for app rounds
func (r *rosenTss) NewMessage(receiverId string, senderId string, message string, messageId string, name string) models.GossipMessage {

	m := models.GossipMessage{
		Message:    message,
		MessageId:  messageId,
		SenderId:   senderId,
		ReceiverId: receiverId,
		Name:       name,
	}

	return m
}
