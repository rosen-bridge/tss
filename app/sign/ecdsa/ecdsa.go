package ecdsa

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	ecdsaSigning "github.com/binance-chain/tss-lib/ecdsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"golang.org/x/crypto/blake2b"
	"math/big"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"strings"
	"time"
)

type operationECDSASign struct {
	sign.OperationSign
	savedData ecdsaKeygen.LocalPartySaveData
}

func NewSignECDSAOperation(signMessage models.SignMessage) _interface.Operation {
	return &operationECDSASign{
		OperationSign: sign.OperationSign{SignMessage: signMessage},
	}
}

// Init initializes the ecdsa sign partyId and creates partyId message
func (s *operationECDSASign) Init(rosenTss _interface.RosenTss, receiverId string) error {

	models.Logger.Info("Init called")

	if s.LocalTssData.PartyID == nil {
		data, pID, err := rosenTss.GetStorage().LoadECDSAKeygen(rosenTss.GetPeerHome())
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
func (s *operationECDSASign) Loop(rosenTss _interface.RosenTss, messageCh chan models.Message) error {

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
				models.Logger.Info("received party message:", fmt.Sprintf("from: %s", msg.SenderId))
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
				models.Logger.Info("received sign message: ", fmt.Sprintf("from: %s", msg.SenderId))
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
					s.LocalTssData.Party = ecdsaSigning.NewLocalParty(signData, s.LocalTssData.Params, s.savedData, outCh, endCh)
				}
				if !s.LocalTssData.Party.Running() {
					go func() {
						if err := s.LocalTssData.Party.Start(); err != nil {
							models.Logger.Error(err)
							errorCh <- err
							return
						}
						models.Logger.Info("party started")
					}()
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
func (s *operationECDSASign) GetClassName() string {
	return "ecdsaSign"
}

// HandleOutMessage handling party messages on out channel
func (s *operationECDSASign) handleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message) error {
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
func (s *operationECDSASign) handleEndMessage(rosenTss _interface.RosenTss, saveData *common.SignatureData) error {

	signData := models.SignData{
		Signature: hex.EncodeToString(saveData.Signature),
		R:         hex.EncodeToString(saveData.R),
		S:         hex.EncodeToString(saveData.S),
		M:         hex.EncodeToString(saveData.M),
	}

	models.Logger.Infof("sign result: R: {%s}, S: {%s}, M:{%s}\n", signData.R, signData.S, signData.M)
	models.Logger.Infof("signature: %v", signData.Signature)

	err := rosenTss.GetConnection().CallBack(s.SignMessage.CallBackUrl, signData)
	if err != nil {
		return err
	}

	return nil

}

// GossipMessageHandler handling all party messages on outCH and endCh
func (s *operationECDSASign) gossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan common.SignatureData) error {
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
func (s *operationECDSASign) partyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id &&
		(gossipMessage.ReceiverId == "" || gossipMessage.ReceiverId == s.LocalTssData.PartyID.Id) {

		models.Logger.Info("received partyId message ", fmt.Sprintf("from: %s", gossipMessage.SenderId))
		partyIdParams := strings.Split(gossipMessage.Message, ",")
		key, _ := new(big.Int).SetString(partyIdParams[2], 10)
		newParty := tss.NewPartyID(partyIdParams[0], partyIdParams[1], key)

		meta := rosenTss.GetMetaData()

		switch partyIdParams[3] {

		case "fromSign":
			if !utils.IsPartyExist(newParty, s.LocalTssData.PartyIds) {
				s.LocalTssData.PartyIds = tss.SortPartyIDs(
					append(s.LocalTssData.PartyIds.ToUnSorted(), newParty))
			}
			if s.LocalTssData.Params == nil {
				if len(s.LocalTssData.PartyIds) >= meta.Threshold {
					err := s.setup(rosenTss)
					if err != nil {
						return err
					}
				} else {
					err := s.Init(rosenTss, "")
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

// PartyUpdate updates partyIds in ecdsa app party based on received message
func (s *operationECDSASign) partyUpdate(partyMsg models.PartyMessage) error {
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
		if s.LocalTssData.PartyID.Index == dest[0].Index {

			models.Logger.Infof("updating party state p2p")
			err := s.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

// Setup called after if Init up was successful. it used to create party params and sign message
func (s *operationECDSASign) setup(rosenTss _interface.RosenTss) error {
	models.Logger.Info("setup called")

	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	meta := rosenTss.GetMetaData()
	models.Logger.Infof("meta %+v", meta)

	if len(s.LocalTssData.PartyIds) < meta.Threshold {
		return fmt.Errorf("not eanough partyId")
	}

	s.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.LocalTssData.PartyIds.ToUnSorted(), s.LocalTssData.PartyID))

	models.Logger.Infof("partyIds {%+v}, local partyId index {%d}", s.LocalTssData.PartyIds, s.LocalTssData.PartyID.Index)

	ctx := tss.NewPeerContext(s.LocalTssData.PartyIds)
	s.LocalTssData.Params = tss.NewParameters(
		tss.S256(), ctx, s.LocalTssData.PartyID, len(s.LocalTssData.PartyIds), meta.Threshold)
	models.Logger.Infof("localECDSAData params: %v\n", *s.LocalTssData.Params)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := hex.EncodeToString(messageBytes[:])
	jsonMessage := rosenTss.NewMessage("", s.LocalTssData.PartyID.Id, "start sign", messageId, "sign")

	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}
