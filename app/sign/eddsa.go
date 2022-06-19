package sign

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaSigning "github.com/binance-chain/tss-lib/eddsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"golang.org/x/crypto/blake2b"
	"math/big"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
	"strings"
	"time"
)

type operationEDDSASign struct {
	OperationSign
	savedData eddsaKeygen.LocalPartySaveData
}

func NewSignEDDSAOperation(signMessage models.SignMessage) _interface.Operation {
	return &operationEDDSASign{
		OperationSign: OperationSign{SignMessage: signMessage},
	}
}

// Init initializes the eddsa sign partyId and creates partyId message
func (s *operationEDDSASign) Init(rosenTss _interface.RosenTss, receiverId string) error {

	models.Logger.Info("Init called")

	if s.LocalTssData.PartyID == nil {
		data, pID, err := rosenTss.GetStorage().LoadEDDSAKeygen(rosenTss.GetPeerHome())
		if err != nil {
			models.Logger.Info(err)
			return err
		}
		if pID == nil {
			models.Logger.Info("pIDs is nil")
			return err
		}
		s.savedData = data
		s.LocalTssData.PartyID = pID
	}
	message := fmt.Sprintf("%s,%s,%d,%s", s.LocalTssData.PartyID.Id, s.LocalTssData.PartyID.Moniker, s.LocalTssData.PartyID.KeyInt(), "fromSign")
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := hex.EncodeToString(signDataBytes[:])
	jsonMessage := rosenTss.NewMessage(receiverId, s.LocalTssData.PartyID.Id, message, messageId, "partyId")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}

// Loop listens to the given channel and parsing the message based on the name
func (s *operationEDDSASign) Loop(rosenTss _interface.RosenTss, messageCh chan models.Message) error {

	models.Logger.Infof("channel", messageCh)
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	errorCh := make(chan error)

	for {
		select {
		case err := <-errorCh:
			return err
		case message, ok := <-messageCh:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			msg := message.Message
			models.Logger.Infof("msg.name: {%s}", msg.Name)
			switch msg.Name {
			case "partyId":
				if msg.Message != "" {
					err := s.partyIdMessageHandler(rosenTss, msg)
					if err != nil {
						return err
					}
				}
			case "partyMsg":
				models.Logger.Info("received party message:",
					fmt.Sprintf("from: %s", msg.SenderId))
				msgBytes, err := hex.DecodeString(msg.Message)
				if err != nil {
					return err
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					return err
				}
				err = s.partyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case "sign":
				models.Logger.Info("received sign message: ",
					fmt.Sprintf("from: %s", msg.SenderId))
				outCh := make(chan tss.Message, len(s.LocalTssData.PartyIds))
				endCh := make(chan common.SignatureData, len(s.LocalTssData.PartyIds))
				for {
					if s.LocalTssData.Params == nil {
						time.Sleep(time.Second)
						continue
					} else {
						break
					}
				}

				if s.LocalTssData.Party == nil {
					s.LocalTssData.Party = eddsaSigning.NewLocalParty(signData, s.LocalTssData.Params, s.savedData, outCh, endCh)
					if err := s.LocalTssData.Party.Start(); err != nil {
						return err
					}
					models.Logger.Info("party started")
					go func() {
						err := s.gossipMessageHandler(rosenTss, outCh, endCh)
						if err != nil {
							models.Logger.Error(err)
							errorCh <- err
							return
						}
					}()
				}
			}
		}
	}
}

// GetClassName returns the class name
func (s *operationEDDSASign) GetClassName() string {
	return "eddsaSign"
}

// HandleOutMessage handling party messages on out channel
func (s *operationEDDSASign) handleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message) error {
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	msgHex, err := s.PartyMessageHandler(partyMsg)
	if err != nil {
		return err
	}
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := hex.EncodeToString(messageBytes[:])
	jsonMessage := rosenTss.NewMessage("", s.LocalTssData.PartyID.Id, msgHex, messageId, "partyMsg")
	err = rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}

// HandleEndMessage handling save data on end cahnnel of party
func (s *operationEDDSASign) handleEndMessage(rosenTss _interface.RosenTss, saveData *common.SignatureData) error {

	sign := models.SignData{
		Signature: hex.EncodeToString(saveData.Signature),
		R:         hex.EncodeToString(saveData.R),
		S:         hex.EncodeToString(saveData.S),
		M:         hex.EncodeToString(saveData.M),
	}

	models.Logger.Infof("sign result: R: {%s}, S: {%s}, M:{%s}\n", sign.R, sign.S, sign.M)
	models.Logger.Infof("signature: %v", sign.Signature)
	models.Logger.Info("EDDSA signing test done.")

	err := rosenTss.GetConnection().CallBack(s.SignMessage.CallBackUrl, sign)
	if err != nil {
		return err
	}

	return nil

}

// GossipMessageHandler handling all party messages on outCH and endCh
func (s *operationEDDSASign) gossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan common.SignatureData) error {
	for {
		select {
		case partyMsg := <-outCh:
			err := s.handleOutMessage(rosenTss, partyMsg)
			if err != nil {
				return err
			}
		case save := <-endCh:
			err := s.handleEndMessage(rosenTss, &save)
			if err != nil {
				return err
			}
		}
	}
}

// PartyIdMessageHandler handles partyId message and if cals setup functions if patryIds list length was at least equal to the threshold
func (s *operationEDDSASign) partyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id &&
		(gossipMessage.ReceiverId == "" || gossipMessage.ReceiverId == s.LocalTssData.PartyID.Id) {

		models.Logger.Info("received partyId message ",
			fmt.Sprintf("from: %s", gossipMessage.SenderId))
		partyIdParams := strings.Split(gossipMessage.Message, ",")
		models.Logger.Infof("partyIdParams: %v", partyIdParams)
		key, _ := new(big.Int).SetString(partyIdParams[2], 10)
		newParty := tss.NewPartyID(partyIdParams[0], partyIdParams[1], key)

		meta := rosenTss.GetMetaData()

		switch partyIdParams[3] {

		case "fromSign":
			if !s.IsExist(newParty, s.LocalTssData.PartyIds) {
				s.LocalTssData.PartyIds = tss.SortPartyIDs(
					append(s.LocalTssData.PartyIds.ToUnSorted(), newParty))

				if len(s.LocalTssData.PartyIds) < meta.Threshold {
					err := s.Init(rosenTss, newParty.Id)
					if err != nil {
						return err
					}
				} else {
					err := s.setup(rosenTss)
					if err != nil {
						return err
					}
				}
			}

		default:
			return fmt.Errorf("wrong message")
		}
	}
	return nil
}

// PartyUpdate updates partyIds in eddsa app party based on received message
func (s *operationEDDSASign) partyUpdate(partyMsg models.PartyMessage) error {
	dest := partyMsg.To
	if dest == nil { // broadcast!

		if s.LocalTssData.Party.PartyID().Index == partyMsg.GetFrom.Index {
			return nil
		}
		models.Logger.Infof("updating party state")
		err := s.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}

	} else { // point-to-point!
		if dest[0].Index == partyMsg.GetFrom.Index {
			err := fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, partyMsg.GetFrom.Index)
			return err
		}
		models.Logger.Infof("updating party state p2p")
		err := s.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

// Setup called after if Init up was successful. it used to create party params and sign message
func (s *operationEDDSASign) setup(rosenTss _interface.RosenTss) error {
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	meta := rosenTss.GetMetaData()

	models.Logger.Infof("meta %+v", meta)

	if len(s.LocalTssData.PartyIds) < meta.Threshold {
		return fmt.Errorf("not eanough partyId")
	}

	s.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.LocalTssData.PartyIds.ToUnSorted(), s.LocalTssData.PartyID))

	models.Logger.Infof("partyIds {%+v}", s.LocalTssData.PartyIds)
	models.Logger.Infof("partyIds count {%+v}", len(s.LocalTssData.PartyIds))

	ctx := tss.NewPeerContext(s.LocalTssData.PartyIds)
	models.Logger.Info("creating params")
	models.Logger.Infof("PartyID: %d, peerId: %s", s.LocalTssData.PartyID.Index, s.LocalTssData.PartyID.Id)

	s.LocalTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, s.LocalTssData.PartyID, len(s.LocalTssData.PartyIds), meta.Threshold)

	models.Logger.Infof("params created: %v", s.LocalTssData.Params.EC().Params().N)
	models.Logger.Infof("localEDDSAData params: %v\n", *s.LocalTssData.Params)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := hex.EncodeToString(messageBytes[:])
	jsonMessage := rosenTss.NewMessage("", s.LocalTssData.PartyID.Id, signData.String(), messageId, "sign")

	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}
