package _interface

import (
	"github.com/binance-chain/tss-lib/tss"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
)

// Operation (sign, keygen , regroup for ecdsa and eddsa protocol)
type Operation interface {
	Init(RosenTss, string) error
	Loop(RosenTss, chan models.Message) error
	PartyIdMessageHandler(rosenTss RosenTss, gossipMessage models.GossipMessage) error
	PartyUpdate(models.PartyMessage) error
	GossipMessageHandler(rosenTss RosenTss, outCh chan tss.Message) error
	GetClassName() string
	//
	// NewMessage(message models.Message)

	// check operations type (sign, regroup, keygen)
	// GetIdentifier() or GetClassName()
}

// RosenTss Interface of an app
type RosenTss interface {
	NewMessage(receiverId string, senderId string, message string, messageId string, name string) models.GossipMessage

	StartNewSign(models.SignMessage) error
	StartNewRegroup(models.RegroupMessage) error
	MessageHandler(models.Message)

	GetStorage() storage.Storage
	GetConnection() network.Connection

	SetMetaData() error
	GetMetaData() models.MetaData

	SetPeerHome(string) error
	GetPeerHome() string

	SetPrivate(models.Private) error
	GetPrivate(string) (string, error)
}
