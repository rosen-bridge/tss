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
	"rosen-bridge/tss/app/keygen"
	"rosen-bridge/tss/app/sign"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
	"strings"
)

const (
	privateFileFormat = "private.txt"
)

type rosenTss struct {
	ChannelMap map[string]chan models.Message
	metaData   models.MetaData
	storage    storage.Storage
	connection network.Connection
	Private    models.Private
	peerHome   string
}

// NewRosenTss Constructor of an app
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
		//TODO: implement this

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
				models.Logger.Errorf("en error occurred in eddsa sign loop, err: %+v", err)
				os.Exit(1)
			}
			models.Logger.Info("end of loop")
		}()
	}
	return nil
}

// StartNewKeygen starts keygen scenario for app based on given protocol.
func (r *rosenTss) StartNewKeygen(keygenMessage models.KeygenMessage) error {
	log.Printf("Starting New keygen process")

	meta := models.MetaData{
		PeersCount: keygenMessage.PeersCount,
		Threshold:  keygenMessage.Threshold,
	}
	err := r.GetStorage().WriteData(meta, r.GetPeerHome(), "config.json", "eddsa")
	if err != nil {
		return err
	}

	r.metaData = meta

	if _, ok := r.ChannelMap["keygen"]; !ok {
		messageCh := make(chan models.Message, 100)
		r.ChannelMap["keygen"] = messageCh
		models.Logger.Infof("creating new channel in StartNewKeygen: %v", "keygen")
	}

	// read loop function
	if keygenMessage.Crypto == "ecdsa" {
		//TODO: implement this

	} else if keygenMessage.Crypto == "eddsa" {
		EDDSAOperation := keygen.NewKeygenEDDSAOperation()
		err := EDDSAOperation.Init(r, "")
		if err != nil {
			return err
		}
		go func() {
			models.Logger.Info("calling loop")
			err := EDDSAOperation.Loop(r, r.ChannelMap["keygen"])
			if err != nil {
				models.Logger.Errorf("en error occurred in eddsa Keygen loop, err: %+v", err)
				os.Exit(1)
			}
			models.Logger.Info("end of loop")
		}()
	}
	return nil
}

// StartNewRegroup starts Regroup scenario for app based on given protocol.
func (r *rosenTss) StartNewRegroup(regroupMessage models.RegroupMessage) error {
	panic("not implemented")
}

// MessageHandler handles the receiving message from message route
func (r *rosenTss) MessageHandler(message models.Message) {

	models.Logger.Infof("new message: %v", message)
	if _, ok := r.ChannelMap[message.Message.MessageId]; !ok {
		models.Logger.Infof("creating new channel in MessageHandler: %v", message.Message.MessageId)
		messageCh := make(chan models.Message, 100)
		r.ChannelMap[message.Message.MessageId] = messageCh
	}
	r.ChannelMap[message.Message.MessageId] <- message
}

// GetStorage returns the storage
func (r *rosenTss) GetStorage() storage.Storage {
	return r.storage
}

// GetConnection returns the connection
func (r *rosenTss) GetConnection() network.Connection {
	return r.connection
}

//SetPeerHome setups peer home address and creates that
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

// GetPeerHome returns the peer's home
func (r *rosenTss) GetPeerHome() string {
	return r.peerHome
}

// SetMetaData setting ups metadata from given file in the home directory
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

// GetMetaData returns peer's meta data
func (r *rosenTss) GetMetaData() models.MetaData {
	return r.metaData
}

// NewMessage creates gossip messages before publish
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

func (r *rosenTss) SetPrivate(private models.Private) error {
	err := r.GetStorage().WriteData(private, r.GetPeerHome(), privateFileFormat, private.Crypto)
	if err != nil {
		return err
	}
	return nil
}

func (r *rosenTss) GetPrivate(crypto string) (string, error) {
	private, err := r.GetStorage().LoadPrivate(r.GetPeerHome(), crypto)
	if err != nil {
		return "", err
	}
	return private, nil
}
