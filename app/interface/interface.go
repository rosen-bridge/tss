package _interface

import (
	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/tss"
	"math/big"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
)

type Operation interface {
	// first initial of tss Operation (sign, keygen , regroup for ecdsa and eddsa)
	Init(RosenTss, string) error
	Loop(RosenTss, chan models.Message) error
	PartyIdMessageHandler(rosenTss RosenTss, gossipMessage models.GossipMessage, signData *big.Int) error
	PartyUpdate(models.PartyMessage) error
	Setup(RosenTss, *big.Int) error
	GossipMessageHandler(rosenTss RosenTss, outCh chan tss.Message, endCh chan common.SignatureData, signData *big.Int) error
	GetClassName() string
	//
	// NewMessage(message models.Message)

	// check operations type (sign, regroup, keygen)
	// GetIdentifier() or GetClassName()
}

// RosenTss Interface of a app
type RosenTss interface {
	NewMessage(receiverId string, senderId string, message string, messageId string, name string) models.GossipMessage

	StartNewSign(models.SignMessage) error
	MessageHandler(models.Message)

	GetStorage() storage.Storage
	GetConnection() network.Connection

	SetMetaData() error
	GetMetaData() models.MetaData

	SetPeerHome(string) error
	GetPeerHome() string
}
