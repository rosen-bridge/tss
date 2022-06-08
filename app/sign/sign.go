package sign

import (
	"encoding/hex"
	"encoding/json"
	"github.com/binance-chain/tss-lib/tss"
	"go.uber.org/zap"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

const (
	signFileName = "sign_data.json"
)

type signOperation struct {
	_interface.Operation
	LocalTssData models.TssData
}

func (s *signOperation) signPartyMessageHandler(partyMsg tss.Message) (string, error) {
	msgBytes, _, err := partyMsg.WireBytes()
	if err != nil {
		models.Logger.Error(err)
		return "", err
	}
	models.Logger.Infof("outch string: {%s}", partyMsg.String())
	partyMessage := models.PartyMessage{
		Message:     msgBytes,
		IsBroadcast: partyMsg.IsBroadcast(),
		GetFrom:     partyMsg.GetFrom(),
		To:          partyMsg.GetTo(),
	}

	partyMessageBytes, err := json.Marshal(partyMessage)
	if err != nil {
		models.Logger.Error("failed to marshal message", zap.Error(err))
		return "", err
	}
	return hex.EncodeToString(partyMessageBytes), nil
}

// sharedPartyUpdater used to update app party
func (s *signOperation) sharedPartyUpdater(party tss.Party, msg models.PartyMessage) error {
	// do not send a message from this party back to itself

	if party.PartyID() == msg.GetFrom {
		return nil
	}
	if _, err := party.UpdateFromBytes(msg.Message, msg.GetFrom, msg.IsBroadcast); err != nil {
		return err
	}
	return nil
}
