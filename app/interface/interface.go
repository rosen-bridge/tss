package _interface

import (
	"encoding/hex"
	"encoding/json"
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
	Loop(RosenTss, chan models.Message, *big.Int) error
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

type OperationHandler struct {
	Operation
}

func (o *OperationHandler) PartyMessageHandler(partyMsg tss.Message) (string, error) {
	msgBytes, _, err := partyMsg.WireBytes()
	if err != nil {
		return "", err
	}
	models.Logger.Infof("outch string: {%s}", partyMsg.String())
	partyMessage := models.PartyMessage{
		Message:                 msgBytes,
		IsBroadcast:             partyMsg.IsBroadcast(),
		GetFrom:                 partyMsg.GetFrom(),
		To:                      partyMsg.GetTo(),
		IsToOldCommittee:        partyMsg.IsToOldCommittee(),
		IsToOldAndNewCommittees: partyMsg.IsToOldAndNewCommittees(),
	}

	partyMessageBytes, err := json.Marshal(partyMessage)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(partyMessageBytes), nil
}

// SharedPartyUpdater used to update app party
func (o *OperationHandler) SharedPartyUpdater(party tss.Party, msg models.PartyMessage) error {
	// do not send a message from this party back to itself

	if party.PartyID() == msg.GetFrom {
		return nil
	}
	if _, err := party.UpdateFromBytes(msg.Message, msg.GetFrom, msg.IsBroadcast); err != nil {
		return err
	}
	return nil
}
