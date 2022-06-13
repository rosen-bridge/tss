package sign

import (
	"encoding/hex"
	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/tss"
	"golang.org/x/crypto/blake2b"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

const (
	signFileName = "sign_data.json"
)

type OperationSign struct {
	_interface.OperationHandler
	LocalTssData models.TssData
	SignMessage  models.SignMessage
	endCh        chan common.SignatureData
}

// GossipMessageHandler called in the main loop of message passing between peers for signing scenario.
func (o *OperationSign) GossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message) error {
	for {
		select {
		case partyMsg := <-outCh:
			signMsgBytes, _ := hex.DecodeString(o.SignMessage.Message)
			signData := new(big.Int).SetBytes(signMsgBytes)
			msgHex, err := o.PartyMessageHandler(partyMsg)
			if err != nil {
				return err
			}
			messageBytes := blake2b.Sum256(signData.Bytes())
			messageId := hex.EncodeToString(messageBytes[:])
			jsonMessage := rosenTss.NewMessage("", o.LocalTssData.PartyID.Id, msgHex, messageId, "partyMsg")
			err = rosenTss.GetConnection().Publish(jsonMessage)
			if err != nil {
				return err
			}
		case save := <-o.endCh:

			models.Logger.Infof("sign result: R: {%s}, S: {%s}, M:{%s}\n", hex.EncodeToString(save.R), hex.EncodeToString(save.S), hex.EncodeToString(save.M))
			models.Logger.Infof("signature: %v", save.Signature)

			err := rosenTss.GetConnection().CallBack(o.SignMessage.CallBackUrl, &save)
			if err != nil {
				return err
			}

			return nil
		}
	}
}
