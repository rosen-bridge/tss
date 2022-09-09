package app

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/labstack/gommon/log"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"rosen-bridge/tss/app/interface"
	ecdsaKeygen "rosen-bridge/tss/app/keygen/ecdsa"
	eddsaKeygen "rosen-bridge/tss/app/keygen/eddsa"
	ecdsaRegroup "rosen-bridge/tss/app/regroup/ecdsa"
	eddsaRegroup "rosen-bridge/tss/app/regroup/eddsa"
	ecdsaSign "rosen-bridge/tss/app/sign/ecdsa"
	eddsaSign "rosen-bridge/tss/app/sign/eddsa"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
	"rosen-bridge/tss/utils"
	"strings"
	"time"
)

const (
	privateFileFormat = "private.txt"
)

type rosenTss struct {
	ChannelMap        map[string]chan models.GossipMessage
	metaData          models.MetaData
	storage           storage.Storage
	connection        network.Connection
	Private           models.Private
	peerHome          string
	operations        []_interface.Operation
	operationsTimeout int
	messageTimeout    int
}

var logging *zap.SugaredLogger

// NewRosenTss Constructor of an app
func NewRosenTss(connection network.Connection, storage storage.Storage, config models.Config) _interface.RosenTss {
	logging = logger.NewSugar("tss")
	return &rosenTss{
		ChannelMap:        make(map[string]chan models.GossipMessage),
		metaData:          models.MetaData{},
		storage:           storage,
		connection:        connection,
		Private:           models.Private{},
		peerHome:          config.HomeAddress,
		operationsTimeout: config.OperationTimeout,
		messageTimeout:    config.MessageTimeout,
	}
}

// StartNewSign starts sign scenario for app based on given protocol.
func (r *rosenTss) StartNewSign(signMessage models.SignMessage) error {

	log.Printf("Starting New Sign process")
	err := r.SetMetaData(signMessage.Crypto)
	if err != nil {
		return err
	}

	msgBytes, _ := hex.DecodeString(signMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	signDataHash := hex.EncodeToString(signDataBytes[:])
	log.Printf("signDtaHash: %v", signDataHash)

	messageId := fmt.Sprintf("%s%s", signMessage.Crypto, signDataHash)
	_, ok := r.ChannelMap[messageId]
	if !ok {
		messageCh := make(chan models.GossipMessage, 100)
		r.ChannelMap[messageId] = messageCh
		logging.Infof("creating new channel in StartNewSign: %v", messageId)
	} else {
		return fmt.Errorf(models.DuplicatedMessageIdError)
	}

	var operation _interface.Operation
	switch signMessage.Crypto {
	case "ecdsa":
		operation = ecdsaSign.NewSignECDSAOperation(signMessage)
	case "eddsa":
		operation = eddsaSign.NewSignEDDSAOperation(signMessage)
	default:
		return fmt.Errorf(models.WrongCryptoProtocolError)
	}
	r.operations = append(r.operations, operation)
	operationState := true

	go func() {
		timeout := time.After(time.Second * time.Duration(r.operationsTimeout))
		for {
			select {
			case <-timeout:
				if _, ok := r.ChannelMap[messageId]; ok {
					close(r.ChannelMap[messageId])
					operationState = false
				}
				return
			}
		}
	}()

	err = operation.Init(r, "")
	if err != nil {
		return err
	}
	go func() {
		logging.Infof("calling loop for %s sign", signMessage.Crypto)
		err = operation.Loop(r, r.ChannelMap[messageId])
		if err != nil {
			if !operationState {
				err = fmt.Errorf("sign operation timeout")
			}
			logging.Errorf("en error occurred in %s sign loop, err: %+v", signMessage.Crypto, err)
			callbackErr := r.GetConnection().CallBack(signMessage.CallBackUrl, err.Error(), "error")
			if callbackErr != nil {
				logging.Error(callbackErr)
			}
		}
		r.deleteInstance(messageId, operation.GetClassName())
		logging.Infof("end of %s sign loop", signMessage.Crypto)
	}()

	return nil
}

// StartNewKeygen starts keygen scenario for app based on given protocol.
func (r *rosenTss) StartNewKeygen(keygenMessage models.KeygenMessage) error {
	log.Printf("Starting New keygen process")

	meta := models.MetaData{
		PeersCount: keygenMessage.PeersCount,
		Threshold:  keygenMessage.Threshold,
	}
	path := fmt.Sprintf("%s/%s/%s", r.GetPeerHome(), keygenMessage.Crypto, "keygen_data.json")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf(models.KeygenFileExistError)
	}

	messageId := fmt.Sprintf("%s%s", keygenMessage.Crypto, "Keygen")
	_, ok := r.ChannelMap[messageId]
	if !ok {
		messageCh := make(chan models.GossipMessage, 100)
		r.ChannelMap[messageId] = messageCh
		logging.Infof("creating new channel in StartNewKeygen: %v", messageId)
	} else {
		return fmt.Errorf(models.DuplicatedMessageIdError)
	}
	err := r.GetStorage().WriteData(meta, r.GetPeerHome(), "config.json", keygenMessage.Crypto)
	if err != nil {
		return err
	}
	r.metaData = meta

	var operation _interface.Operation

	switch keygenMessage.Crypto {
	case "ecdsa":
		operation = ecdsaKeygen.NewKeygenECDSAOperation(keygenMessage)
	case "eddsa":
		operation = eddsaKeygen.NewKeygenEDDSAOperation(keygenMessage)
	default:
		return fmt.Errorf(models.WrongCryptoProtocolError)
	}

	r.operations = append(r.operations, operation)

	err = operation.Init(r, "")
	if err != nil {
		return err
	}
	go func() {
		logging.Infof("calling loop for %s keygen", keygenMessage.Crypto)
		err = operation.Loop(r, r.ChannelMap[messageId])
		if err != nil {
			logging.Errorf("en error occurred in %s keygen loop, err: %+v", keygenMessage.Crypto, err)
			callbackErr := r.GetConnection().CallBack(keygenMessage.CallBackUrl, err.Error(), "error")
			if callbackErr != nil {
				logging.Error(callbackErr)
			}
		}
		r.deleteInstance(messageId, operation.GetClassName())
		logging.Infof("end of %s keygen loop", keygenMessage.Crypto)
	}()

	return nil
}

// StartNewRegroup starts Regroup scenario for app based on given protocol.
func (r *rosenTss) StartNewRegroup(regroupMessage models.RegroupMessage) error {
	log.Printf("Starting New regroup process")

	messageId := fmt.Sprintf("%s%s", regroupMessage.Crypto, "Regroup")

	_, ok := r.ChannelMap[messageId]
	if !ok {
		messageCh := make(chan models.GossipMessage, 100)
		r.ChannelMap[messageId] = messageCh
		logging.Infof("creating new channel in StartNewRegroup: %v", messageId)
	} else {
		return fmt.Errorf(models.DuplicatedMessageIdError)
	}
	var operation _interface.Operation

	switch regroupMessage.Crypto {
	case "ecdsa":
		operation = ecdsaRegroup.NewRegroupECDSAOperation(regroupMessage)
	case "eddsa":
		operation = eddsaRegroup.NewRegroupEDDSAOperation(regroupMessage)
	default:
		return fmt.Errorf(models.WrongCryptoProtocolError)
	}
	r.operations = append(r.operations, operation)

	err := operation.Init(r, "")
	if err != nil {
		return err
	}
	go func() {
		logging.Infof("calling loop for %s regroup", regroupMessage.Crypto)
		err = operation.Loop(r, r.ChannelMap[messageId])
		if err != nil {
			logging.Errorf("en error occurred in %s regroup loop, err: %+v", regroupMessage.Crypto, err)
			callbackErr := r.GetConnection().CallBack(regroupMessage.CallBackUrl, err.Error(), "error")
			if callbackErr != nil {
				logging.Error(callbackErr)
			}
		}
		r.deleteInstance(messageId, operation.GetClassName())
		logging.Infof("end of %s regroup loop", regroupMessage.Crypto)
	}()

	return nil
}

// MessageHandler handles the receiving message from message route
func (r *rosenTss) MessageHandler(message models.Message) error {

	msgBytes := []byte(message.Message)
	gossipMsg := models.GossipMessage{}
	err := json.Unmarshal(msgBytes, &gossipMsg)
	if err != nil {
		return err
	}

	logging.Infof("new message: %+v", gossipMsg.Name)

	timeout := time.After(time.Second * time.Duration(r.messageTimeout))
	var state bool

timoutLoop:
	for {
		select {
		case <-timeout:
			logging.Error("timeout")
			state = false
			break timoutLoop
		default:
			if _, ok := r.ChannelMap[gossipMsg.MessageId]; ok {
				r.ChannelMap[gossipMsg.MessageId] <- gossipMsg
				state = true
				break timoutLoop
			}
		}
	}

	if !state {
		return fmt.Errorf("channel not found: %+v", gossipMsg.MessageId)
	} else {
		return nil
	}
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
	logging.Info("setting up home directory")

	absAddress, err := utils.GetAbsoluteAddress(homeAddress)
	if err != nil {
		return err
	}
	r.peerHome = absAddress

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
func (r *rosenTss) SetMetaData(crypto string) error {
	// locating file
	var configFile string
	rootFolder := filepath.Join(r.peerHome, crypto)
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
	logging.Infof("File: %v", filePath)

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

// SetPrivate writes private data in a file in peer home folder
func (r *rosenTss) SetPrivate(private models.Private) error {
	err := r.GetStorage().WriteData(private, r.GetPeerHome(), privateFileFormat, private.Crypto)
	if err != nil {
		return err
	}
	return nil
}

// GetPrivate returns private data from peer home folder
func (r *rosenTss) GetPrivate(crypto string) string {
	private := r.GetStorage().LoadPrivate(r.GetPeerHome(), crypto)
	return private
}

// GetOperations returns list of operations
func (r *rosenTss) GetOperations() []_interface.Operation {
	return r.operations
}

// deleteInstance removes operation and related channel from list
func (r *rosenTss) deleteInstance(channelId string, operationName string) {
	for i, operation := range r.operations {
		if operation.GetClassName() == operationName {
			r.operations = append(r.operations[:i], r.operations[i+1:]...)
		}
	}
	delete(r.ChannelMap, channelId)
}

// GetPublicKey returns generated pk in the keygen process
func (r *rosenTss) GetPublicKey(crypto string) (string, error) {
	switch crypto {
	case "ecdsa":
		saveData, _, err := r.GetStorage().LoadECDSAKeygen(r.peerHome)
		if err != nil {
			return "", err
		}
		pkX, pkY := saveData.ECDSAPub.X(), saveData.ECDSAPub.Y()
		pk := ecdsa.PublicKey{
			Curve: tss.S256(),
			X:     pkX,
			Y:     pkY,
		}

		public := utils.GetPKFromECDSAPub(pk.X, pk.Y)
		hexPk := hex.EncodeToString(public)
		return hexPk, nil
	case "eddsa":
		saveData, _, err := r.GetStorage().LoadEDDSAKeygen(r.peerHome)
		if err != nil {
			return "", err
		}
		pkX, pkY := saveData.EDDSAPub.X(), saveData.EDDSAPub.Y()
		pk := edwards.PublicKey{
			Curve: tss.Edwards(),
			X:     pkX,
			Y:     pkY,
		}

		public := utils.GetPKFromEDDSAPub(pk.X, pk.Y)
		hexPk := hex.EncodeToString(public)
		return hexPk, nil
	default:
		return "", fmt.Errorf(models.WrongCryptoProtocolError)
	}
}
