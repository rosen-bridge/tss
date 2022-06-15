package _interface

import (
	"encoding/hex"
	"encoding/json"
	"github.com/binance-chain/tss-lib/tss"
	"rosen-bridge/tss/models"
)

type OperationHandler struct {
	Operation
}

// PartyMessageHandler handles gossip message from party to party(s)
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
