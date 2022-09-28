package ecdsa

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	ecdsaSigning "github.com/binance-chain/tss-lib/ecdsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
	"math/big"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"time"
)

type operationECDSASign struct {
	operationSign sign.OperationSign
	savedData     ecdsaKeygen.LocalPartySaveData
}

var logging *zap.SugaredLogger

func NewSignECDSAOperation(signMessage models.SignMessage) _interface.Operation {
	logging = logger.NewSugar("ecdsa-sign")
	return &operationECDSASign{
		operationSign: sign.OperationSign{
			SignMessage: signMessage,
			PeersMap:    make(map[string]string),
		},
	}
}

// Init initializes the ecdsa sign partyId and creates partyId message
func (s *operationECDSASign) Init(rosenTss _interface.RosenTss, receiverId string) error {

	logging.Info("Init called")

	if s.operationSign.LocalTssData.PartyID == nil {
		data, pID, err := rosenTss.GetStorage().LoadECDSAKeygen(rosenTss.GetPeerHome())
		if err != nil {
			logging.Error(err)
			return err
		}
		if pID == nil {
			logging.Error("pIDs is nil")
			return err
		}
		s.savedData = data
		s.operationSign.LocalTssData.PartyID = pID
	}

	var noAnswer bool
	if receiverId == "" {
		noAnswer = false
	} else {
		noAnswer = true
	}
	message := models.Register{
		Id:        s.operationSign.LocalTssData.PartyID.Id,
		Moniker:   s.operationSign.LocalTssData.PartyID.Moniker,
		Key:       s.operationSign.LocalTssData.PartyID.KeyInt().String(),
		Timestamp: time.Now().Unix() / 60,
		NoAnswer:  noAnswer,
	}
	marshal, err := json.Marshal(message)
	if err != nil {
		return err
	}

	msgBytes, _ := hex.DecodeString(s.operationSign.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "ecdsa", hex.EncodeToString(signDataBytes[:]))
	err = s.newMessage(rosenTss, receiverId, s.operationSign.LocalTssData.PartyID.Id, string(marshal), messageId, "register")
	if err != nil {
		return err
	}
	return nil
}

// Loop listens to the given channel and parsing the message based on the name
func (s *operationECDSASign) Loop(rosenTss _interface.RosenTss, messageCh chan models.GossipMessage) error {

	msgBytes, _ := hex.DecodeString(s.operationSign.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	errorCh := make(chan error)

	for {
		select {
		case err := <-errorCh:
			if err.Error() == "close channel" {
				close(messageCh)
				return nil
			}
			return err
		case msg, ok := <-messageCh:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			logging.Infof("msg.name: {%s}", msg.Name)
			err := s.verify(msg)
			if err != nil {
				return err
			}
			switch msg.Name {
			case "register":
				if msg.Message != "" {
					err := s.registerMessageHandler(rosenTss, msg)
					if err != nil {
						return err
					}
				}
			case "partyMsg":
				logging.Info("received party message:", fmt.Sprintf("from: %s", msg.SenderId))
				msgBytes, err := hex.DecodeString(msg.Message)
				if err != nil {
					return err
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					return err
				}
				if s.operationSign.LocalTssData.Params == nil {
					return fmt.Errorf("this peer is no longer needed. the signing process has been started with the required peers")
				}
				err = s.partyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case "sign":
				logging.Info("received sign message: ", fmt.Sprintf("from: %s", msg.SenderId))
				outCh := make(chan tss.Message, len(s.operationSign.LocalTssData.PartyIds))
				endCh := make(chan common.SignatureData, len(s.operationSign.LocalTssData.PartyIds))
				for {
					if s.operationSign.LocalTssData.Params == nil {
						time.Sleep(time.Second)
						continue
					} else {
						break
					}
				}

				if s.operationSign.LocalTssData.Party == nil {
					s.operationSign.LocalTssData.Party = ecdsaSigning.NewLocalParty(signData, s.operationSign.LocalTssData.Params, s.savedData, outCh, endCh)
				}
				if !s.operationSign.LocalTssData.Party.Running() {
					go func() {
						if err := s.operationSign.LocalTssData.Party.Start(); err != nil {
							logging.Error(err)
							errorCh <- err
							return
						}
						logging.Info("party started")
					}()
					go func() {
						result, err := s.gossipMessageHandler(rosenTss, outCh, endCh)
						if err != nil {
							logging.Error(err)
							errorCh <- err
							return
						}
						if result {
							errorCh <- fmt.Errorf("close channel")
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
	msgBytes, _ := hex.DecodeString(s.operationSign.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	msgHex, err := s.operationSign.OperationHandler.PartyMessageHandler(partyMsg)
	if err != nil {
		return err
	}
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "ecdsa", hex.EncodeToString(messageBytes[:]))
	err = s.newMessage(rosenTss, "", s.operationSign.LocalTssData.PartyID.Id, msgHex, messageId, "partyMsg")
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

	logging.Infof("sign result: R: {%s}, S: {%s}, M:{%s}\n", signData.R, signData.S, signData.M)
	logging.Infof("signature: %v", signData.Signature)
	logging.Info("ECDSA signing done.")

	err := rosenTss.GetConnection().CallBack(s.operationSign.SignMessage.CallBackUrl, signData, "ok")
	if err != nil {
		return err
	}

	return nil

}

// GossipMessageHandler handling all party messages on outCH and endCh
func (s *operationECDSASign) gossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan common.SignatureData) (bool, error) {
	for {
		select {
		case partyMsg := <-outCh:
			err := s.handleOutMessage(rosenTss, partyMsg)
			if err != nil {
				return false, err
			}
		case save := <-endCh:
			err := s.handleEndMessage(rosenTss, &save)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

// registerMessageHandler handles register message, and it calls setup functions if patryIds list length was at least equal to the threshold
func (s *operationECDSASign) registerMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {

	if gossipMessage.SenderId != s.operationSign.LocalTssData.PartyID.Id {
		if gossipMessage.ReceiverId != "" {
			s.operationSign.PeersMap[s.operationSign.LocalTssData.PartyID.Id] = gossipMessage.ReceiverId
		}
		s.operationSign.PeersMap[gossipMessage.SenderId] = gossipMessage.SenderP2PId
		logging.Info("received register message ",
			fmt.Sprintf("from: %s", gossipMessage.SenderId))

		registerMessage := &models.Register{}
		err := json.Unmarshal([]byte(gossipMessage.Message), registerMessage)
		if err != nil {
			return err
		}

		logging.Infof("registerMessage: %+v", registerMessage)
		key, _ := new(big.Int).SetString(registerMessage.Key, 10)
		newParty := tss.NewPartyID(registerMessage.Id, registerMessage.Moniker, key)

		meta := rosenTss.GetMetaData()

		if !registerMessage.NoAnswer {
			err := s.Init(rosenTss, gossipMessage.SenderP2PId)
			if err != nil {
				return err
			}
		}

		if !utils.IsPartyExist(newParty, s.operationSign.LocalTssData.PartyIds) {
			s.operationSign.LocalTssData.PartyIds = tss.SortPartyIDs(
				append(s.operationSign.LocalTssData.PartyIds.ToUnSorted(), newParty))
			if len(s.operationSign.LocalTssData.PartyIds) >= meta.Threshold {
				err = s.setup(rosenTss)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// PartyUpdate updates partyIds in ecdsa app party based on received message
func (s *operationECDSASign) partyUpdate(partyMsg models.PartyMessage) error {
	dest := partyMsg.To
	if dest == nil { // broadcast!

		if s.operationSign.LocalTssData.Party.PartyID().Index == partyMsg.GetFrom.Index {
			return nil
		}
		logging.Infof("updating party state")
		err := s.operationSign.OperationHandler.SharedPartyUpdater(s.operationSign.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}

	} else { // point-to-point!
		if dest[0].Index == partyMsg.GetFrom.Index {
			err := fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, partyMsg.GetFrom.Index)
			return err
		}
		if s.operationSign.LocalTssData.PartyID.Index == dest[0].Index {

			logging.Infof("updating party state p2p")
			err := s.operationSign.OperationHandler.SharedPartyUpdater(s.operationSign.LocalTssData.Party, partyMsg)
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
	logging.Info("setup called")

	msgBytes, _ := hex.DecodeString(s.operationSign.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	meta := rosenTss.GetMetaData()
	logging.Infof("meta %+v", meta)

	s.operationSign.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.operationSign.LocalTssData.PartyIds.ToUnSorted(), s.operationSign.LocalTssData.PartyID))

	logging.Infof("partyIds {%+v}, local partyId index {%d}", s.operationSign.LocalTssData.PartyIds, s.operationSign.LocalTssData.PartyID.Index)

	ctx := tss.NewPeerContext(s.operationSign.LocalTssData.PartyIds)

	logging.Info("creating params")
	s.operationSign.LocalTssData.Params = tss.NewParameters(
		tss.S256(), ctx, s.operationSign.LocalTssData.PartyID, len(s.operationSign.LocalTssData.PartyIds), meta.Threshold)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", "ecdsa", hex.EncodeToString(messageBytes[:]))
	err := s.newMessage(rosenTss, "", s.operationSign.LocalTssData.PartyID.Id, "start sign", messageId, "sign")
	if err != nil {
		return err
	}
	return nil
}

func (s *operationECDSASign) signMessage(message []byte) ([]byte, error) {
	private := new(ecdsa.PrivateKey)
	private.PublicKey.Curve = tss.S256()
	private.D = s.savedData.Xi

	var index int
	for i, k := range s.savedData.Ks {
		if s.savedData.ShareID == k {
			index = i
		}
	}

	private.PublicKey.X, private.PublicKey.Y = s.savedData.BigXj[index].X(), s.savedData.BigXj[index].Y()

	checksum := blake2b.Sum256(message)
	signature, err := private.Sign(rand.Reader, checksum[:], nil)
	if err != nil {
		panic(err)
	}
	return signature, nil
}

func (s *operationECDSASign) verify(msg models.GossipMessage) error {
	var index int
	for _, peer := range s.operationSign.LocalTssData.PartyIds {
		if peer.Id == msg.SenderId {
			for i, k := range s.savedData.Ks {
				key := new(big.Int).SetBytes(peer.Key)
				if key.Cmp(k) == 0 {
					index = i
				}
			}
		}
	}
	public := new(ecdsa.PublicKey)
	public.Curve = tss.S256()
	public.X = s.savedData.BigXj[index].X()
	public.Y = s.savedData.BigXj[index].Y()

	payload := models.Payload{
		Message:    msg.Message,
		MessageId:  msg.MessageId,
		SenderId:   msg.SenderId,
		ReceiverId: msg.ReceiverId,
		Name:       msg.Name,
	}
	marshal, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	checksum := blake2b.Sum256(marshal)

	result := ecdsa.VerifyASN1(public, checksum[:], msg.Signature)
	if !result {
		return fmt.Errorf("can not verify the message")
	}

	return nil
}

func (s *operationECDSASign) newMessage(rosenTss _interface.RosenTss, receiverId string, senderId string, message string, messageId string, name string) error {
	payload := models.Payload{
		Message:    message,
		MessageId:  messageId,
		SenderId:   senderId,
		ReceiverId: receiverId,
		Name:       name,
	}
	marshal, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	signature, err := s.signMessage(marshal)
	if err != nil {
		return err
	}
	gossipMessage := rosenTss.NewMessage(receiverId, senderId, message, messageId, name)
	gossipMessage.Signature = signature

	err = rosenTss.GetConnection().Publish(gossipMessage)
	if err != nil {
		return err
	}
	return nil
}
