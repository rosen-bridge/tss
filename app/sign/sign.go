package sign

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/tss"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"time"
)

type OperationSign struct {
	_interface.OperationHandler
	LocalTssData     models.TssData
	SignMessage      models.SignMessage
	PeersMap         map[string]string
	Signatures       map[string]string
	Logger           *zap.SugaredLogger
	SetupSignMessage models.SetupSign
}

// PartyUpdate updates partyIds in eddsa app party based on received message
func (s *OperationSign) PartyUpdate(partyMsg models.PartyMessage) error {
	dest := partyMsg.To
	if dest == nil { // broadcast!

		if s.LocalTssData.Party.PartyID().Index == partyMsg.GetFrom.Index {
			return nil
		}
		s.Logger.Infof("updating party state")
		err := s.OperationHandler.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}

	} else { // point-to-point!
		if dest[0].Index == partyMsg.GetFrom.Index {
			err := fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, partyMsg.GetFrom.Index)
			return err
		}
		s.Logger.Infof("updating party state p2p")
		err := s.OperationHandler.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

// HandleOutMessage handling party messages on out channel
func (s *OperationSign) HandleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message, signer func(message []byte) ([]byte, error)) error {
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	msgHex, err := s.OperationHandler.PartyMessageHandler(partyMsg)
	if err != nil {
		return err
	}
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, hex.EncodeToString(messageBytes[:]))
	payload := models.Payload{
		Message:    msgHex,
		MessageId:  messageId,
		SenderId:   s.LocalTssData.PartyID.Id,
		ReceiverId: "",
		Name:       "partyMsg",
	}
	err = s.NewMessage(rosenTss, payload, signer)
	if err != nil {
		return err
	}
	return nil
}

// HandleEndMessage handling save data on end cahnnel of party
func (s *OperationSign) HandleEndMessage(rosenTss _interface.RosenTss, signatureData *common.SignatureData) error {

	signData := models.SignData{
		Signature: hex.EncodeToString(signatureData.Signature),
		R:         hex.EncodeToString(signatureData.R),
		S:         hex.EncodeToString(signatureData.S),
		M:         hex.EncodeToString(signatureData.M),
	}

	s.Logger.Infof("signData result: R: {%s}, S: {%s}, M:{%s}\n", signData.R, signData.S, signData.M)
	s.Logger.Infof("signature: %v", signData.Signature)
	s.Logger.Infof("%s signing done.", s.SignMessage.Crypto)

	err := rosenTss.GetConnection().CallBack(s.SignMessage.CallBackUrl, signData, "ok")
	if err != nil {
		return err
	}

	return nil

}

// GossipMessageHandler handling all party messages on outCH and endCh
func (s *OperationSign) GossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan common.SignatureData, signer func(message []byte) ([]byte, error)) (bool, error) {
	for {
		select {
		case partyMsg := <-outCh:
			err := s.HandleOutMessage(rosenTss, partyMsg, signer)
			if err != nil {
				return false, err
			}
		case save := <-endCh:
			err := s.HandleEndMessage(rosenTss, &save)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

func (s *OperationSign) NewRegister(rosenTss _interface.RosenTss, receiverId string, signer func(message []byte) ([]byte, error)) error {
	var noAnswer bool
	for _, value := range s.PeersMap {
		if value == receiverId {
			noAnswer = true
		}
	}

	message := models.Register{
		Id:        s.LocalTssData.PartyID.Id,
		Moniker:   s.LocalTssData.PartyID.Moniker,
		Key:       s.LocalTssData.PartyID.KeyInt().String(),
		Timestamp: time.Now().Unix() / 60,
		NoAnswer:  noAnswer,
	}
	marshal, err := json.Marshal(message)
	if err != nil {
		return err
	}

	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, hex.EncodeToString(signDataBytes[:]))
	payload := models.Payload{
		Message:    string(marshal),
		MessageId:  messageId,
		SenderId:   s.LocalTssData.PartyID.Id,
		ReceiverId: receiverId,
		Name:       "register",
	}
	err = s.NewMessage(rosenTss, payload, signer)
	if err != nil {
		return err
	}
	return nil

}

// RegisterMessageHandler handles register message, and it calls setup functions if patryIds list length was at least equal to the threshold
func (s *OperationSign) RegisterMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage, ks []*big.Int, shareID *big.Int, signer func(message []byte) ([]byte, error)) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {
		if gossipMessage.ReceiverId != "" {
			s.PeersMap[s.LocalTssData.PartyID.Id] = gossipMessage.ReceiverId
		}
		s.PeersMap[gossipMessage.SenderId] = gossipMessage.SenderP2PId
		s.Logger.Info("received register message ",
			fmt.Sprintf("from: %s", gossipMessage.SenderId))

		registerMessage := &models.Register{}
		err := json.Unmarshal([]byte(gossipMessage.Message), registerMessage)
		if err != nil {
			return err
		}

		s.Logger.Infof("registerMessage: %+v", registerMessage)
		key, _ := new(big.Int).SetString(registerMessage.Key, 10)
		newParty := tss.NewPartyID(registerMessage.Id, registerMessage.Moniker, key)

		if !registerMessage.NoAnswer {
			err := s.NewRegister(rosenTss, gossipMessage.SenderP2PId, signer)
			if err != nil {
				return err
			}
		}

		if !utils.IsPartyExist(newParty, s.LocalTssData.PartyIds) {
			s.LocalTssData.PartyIds = tss.SortPartyIDs(
				append(s.LocalTssData.PartyIds.ToUnSorted(), newParty))
		}
	}
	return nil
}

// Setup called after if Init up was successful. it used to create party params and sign message
func (s *OperationSign) Setup(rosenTss _interface.RosenTss, signer func(message []byte) ([]byte, error)) error {
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	meta := rosenTss.GetMetaData()
	s.Logger.Infof("meta %+v", meta)

	s.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.LocalTssData.PartyIds.ToUnSorted(), s.LocalTssData.PartyID))

	s.Logger.Infof("partyIds {%+v}, local partyId index {%d}", s.LocalTssData.PartyIds, s.LocalTssData.PartyID.Index)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, hex.EncodeToString(messageBytes[:]))

	setupMessage := models.SetupSign{
		Hash:      hex.EncodeToString(messageBytes[:]),
		Peers:     s.LocalTssData.PartyIds,
		Timestamp: time.Now().Unix() / 60,
		StarterId: s.LocalTssData.PartyID,
	}
	marshal, err := json.Marshal(setupMessage)
	if err != nil {
		return err
	}

	payload := models.Payload{
		Message:    string(marshal),
		MessageId:  messageId,
		SenderId:   s.LocalTssData.PartyID.Id,
		ReceiverId: "",
		Name:       "setup",
	}
	err = s.NewMessage(rosenTss, payload, signer)
	if err != nil {
		return err
	}
	return nil
}

func (s *OperationSign) NewMessage(rosenTss _interface.RosenTss, payload models.Payload, signer func(message []byte) ([]byte, error)) error {

	marshal, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	signature, err := signer(marshal)
	if err != nil {
		return err
	}

	gossipMessage := models.GossipMessage{
		Message:    payload.Message,
		MessageId:  payload.MessageId,
		SenderId:   payload.SenderId,
		ReceiverId: payload.ReceiverId,
		Name:       payload.Name,
		Signature:  signature,
	}

	err = rosenTss.GetConnection().Publish(gossipMessage)
	if err != nil {
		return err
	}
	return nil
}

// SetupMessageHandler handles setup message
func (s *OperationSign) SetupMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage, ks []*big.Int, signer func(message []byte) ([]byte, error)) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {

		setupSignMessage := &models.SetupSign{}
		err := json.Unmarshal([]byte(gossipMessage.Message), setupSignMessage)
		if err != nil {
			return err
		}

		s.Logger.Infof("setup Message data: %+v", setupSignMessage)

		index := utils.IndexOf(ks, setupSignMessage.StarterId.KeyInt())
		minutes := time.Now().Unix() / 60
		if minutes%int64(len(ks)) != int64(index) {
			s.Logger.Errorf("it's not %s turn.", gossipMessage.SenderId)
			return nil
		}

		for _, peer := range setupSignMessage.Peers {
			if !utils.IsPartyExist(peer, s.LocalTssData.PartyIds) {
				receiverId := setupSignMessage.PeersMap[peer.Id]
				err := s.NewRegister(rosenTss, receiverId, signer)
				if err != nil {
					return err
				}
			}
		}
		s.SetupSignMessage = *setupSignMessage
	}
	return nil
}

func (s *OperationSign) SignStarter(rosenTss _interface.RosenTss, senderId string, signer func(message []byte) ([]byte, error)) error {
	msgBytes, _ := hex.DecodeString(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())

	if s.SetupSignMessage.Hash != hex.EncodeToString(signDataBytes[:]) {
		return fmt.Errorf("wrogn hash to sign\nreceivedHash: %s\nexpectedHash: %s", s.SetupSignMessage.Hash, hex.EncodeToString(signDataBytes[:]))
	}

	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, hex.EncodeToString(signDataBytes[:]))

	payload := models.Payload{
		Message:    s.SetupSignMessage.Hash,
		MessageId:  messageId,
		SenderId:   s.LocalTssData.PartyID.Id,
		ReceiverId: senderId,
		Name:       "sign",
	}
	err := s.NewMessage(rosenTss, payload, signer)
	if err != nil {
		return err
	}
	s.SetupSignMessage = models.SetupSign{}
	return nil
}
