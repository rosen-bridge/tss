package _interface

import (
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
)

// Operation (sign, keygen , regroup for ecdsa and eddsa protocol)
type Operation interface {
	Init(RosenTss, string) error
	Loop(RosenTss, chan models.GossipMessage) error
	GetClassName() string
}

// RosenTss Interface of an app
type RosenTss interface {
	NewMessage(receiverId string, senderId string, message string, messageId string, name string) models.GossipMessage

	StartNewSign(models.SignMessage) error
	StartNewRegroup(models.RegroupMessage) error
	StartNewKeygen(models.KeygenMessage) error
	MessageHandler(models.Message) error

	GetStorage() storage.Storage
	GetConnection() network.Connection

	SetMetaData(string) error
	GetMetaData() models.MetaData

	SetPeerHome(string) error
	GetPeerHome() string

	SetPrivate(private models.Private) error
	GetPrivate(string) string

	GetOperations() []Operation
	GetPublicKey(crypto string) (string, error)

	SetP2pId() error
	GetP2pId() string
	GetConfig() models.Config
}
