package sign

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/binance-chain/tss-lib/common"
	"github.com/binance-chain/tss-lib/tss"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
)

const (
	registerMessage  = "register"
	setupMessage     = "setup"
	signMessage      = "sign"
	partyMessage     = "party"
	startSignMessage = "startSign"
)

type Handler interface {
	Sign([]byte) ([]byte, error)
	Verify(models.GossipMessage) error
	MessageHandler(_interface.RosenTss, models.GossipMessage, string, models.TssData, *OperationSign) error
	LoadData(_interface.RosenTss) (*tss.PartyID, error)
	GetData() ([]*big.Int, *big.Int)
}

type OperationSign struct {
	_interface.OperationHandler
	LocalTssData     models.TssData
	SignMessage      models.SignMessage
	Signatures       map[string][]byte
	Logger           *zap.SugaredLogger
	SetupSignMessage models.SetupSign
	Handler
}

// Init initializes the eddsa sign partyId and creates partyId message
func (s *OperationSign) Init(rosenTss _interface.RosenTss, receiverId string) error {

	s.Logger.Info("Init called")

	pID, err := s.LoadData(rosenTss)
	if err != nil {
		s.Logger.Error(err)
		return err
	}
	s.LocalTssData.PartyID = pID

	err = s.NewRegister(rosenTss, "")
	if err != nil {
		return err
	}
	return nil
}

func (s *OperationSign) Loop(rosenTss _interface.RosenTss, messageCh chan models.GossipMessage) error {

	errorCh := make(chan error)
	meta := rosenTss.GetMetaData()

	go func() {
		ks, sharedId := s.GetData()
		err := s.SetupThread(rosenTss, ks, sharedId, meta.Threshold)
		if err != nil {
			errorCh <- err
		}
	}()

	go func() {
		err := s.SignStarterThread(rosenTss)
		if err != nil {
			errorCh <- err
		}
	}()

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
			s.Logger.Infof("msg.name: {%s}", msg.Name)
			err := s.Verify(msg)
			if err != nil {
				return err
			}

			switch msg.Name {
			case registerMessage:
				if msg.Message != "" && s.LocalTssData.Party != nil {
					err := s.RegisterMessageHandler(rosenTss, msg)
					if err != nil {
						return err
					}
				}
			case setupMessage:
				if msg.Message != "" && s.LocalTssData.Party != nil {
					ks, _ := s.GetData()
					err := s.SetupMessageHandler(rosenTss, msg, ks)
					if err != nil {
						return err
					}
				}
			case signMessage:
				if msg.Message != "" && s.LocalTssData.Party != nil {
					ks, sharedId := s.GetData()
					err := s.SignMessageHandler(rosenTss, msg, ks, sharedId)
					if err != nil {
						return err
					}

				}
			case partyMessage:
				s.Logger.Info(
					"received party message:",
					fmt.Sprintf("from: %s", msg.SenderId),
				)
				msgBytes, err := utils.Decoder(msg.Message)
				if err != nil {
					return err
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					return err
				}
				if s.LocalTssData.Params == nil {
					return fmt.Errorf("this peer is no longer needed. the signing process has been started with the required peers")
				}
				err = s.PartyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case startSignMessage:
				if msg.Message != "" && s.LocalTssData.Party != nil {
					go func() {
						err := s.MessageHandler(rosenTss, msg, s.SignMessage.Message, s.LocalTssData, s)
						if err != nil {
							s.Logger.Errorf("StartSignHandler returns error: %+v", err)
							errorCh <- err
						}
					}()
				}
			}
		}
	}
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
func (s *OperationSign) HandleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message) error {
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	msgHex, err := s.OperationHandler.PartyMessageHandler(partyMsg)
	if err != nil {
		return err
	}
	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(messageBytes[:]))
	payload := models.Payload{
		Message:   msgHex,
		MessageId: messageId,
		SenderId:  s.LocalTssData.PartyID.Id,
		Name:      partyMessage,
	}
	err = s.NewMessage(rosenTss, payload, "")
	if err != nil {
		return err
	}
	return nil
}

// HandleEndMessage handling save data on end cahnnel of party
func (s *OperationSign) HandleEndMessage(rosenTss _interface.RosenTss, signatureData *common.SignatureData) error {

	signData := models.SignData{
		Signature: utils.Encoder(signatureData.Signature),
		R:         utils.Encoder(signatureData.R),
		S:         utils.Encoder(signatureData.S),
		M:         utils.Encoder(signatureData.M),
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
func (s *OperationSign) GossipMessageHandler(
	rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan common.SignatureData,
) (bool, error) {
	for {
		select {
		case partyMsg := <-outCh:
			err := s.HandleOutMessage(rosenTss, partyMsg)
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

func (s *OperationSign) NewRegister(rosenTss _interface.RosenTss, receiverId string) error {

	s.Logger.Info("newRegister called")

	var noAnswer bool

	for _, partyId := range s.LocalTssData.PartyIds {
		if partyId.Id == receiverId {
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

	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(signDataBytes[:]))
	payload := models.Payload{
		Message:   string(marshal),
		MessageId: messageId,
		SenderId:  s.LocalTssData.PartyID.Id,
		Name:      registerMessage,
	}
	err = s.NewMessage(rosenTss, payload, receiverId)
	if err != nil {
		return err
	}
	return nil

}

// RegisterMessageHandler handles register message, and it calls setup functions if patryIds list length was at least equal to the threshold
func (s *OperationSign) RegisterMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {
		s.Logger.Info(
			"received register message ",
			fmt.Sprintf("from: %s", gossipMessage.SenderId),
		)

		registerMessage := &models.Register{}
		err := json.Unmarshal([]byte(gossipMessage.Message), registerMessage)
		if err != nil {
			return err
		}

		s.Logger.Infof("registerMessage: %+v", registerMessage)
		key, _ := new(big.Int).SetString(registerMessage.Key, 10)
		newParty := tss.NewPartyID(registerMessage.Id, registerMessage.Moniker, key)

		if !registerMessage.NoAnswer {
			err := s.NewRegister(rosenTss, gossipMessage.SenderId)
			if err != nil {
				return err
			}
		}

		if !utils.IsPartyExist(newParty, s.LocalTssData.PartyIds) {
			s.LocalTssData.PartyIds = tss.SortPartyIDs(
				append(s.LocalTssData.PartyIds.ToUnSorted(), newParty),
			)
		}
	}
	return nil
}

// Setup called after if Init up was successful. it used to create party params and sign message
func (s *OperationSign) Setup(rosenTss _interface.RosenTss) error {
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	s.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.LocalTssData.PartyIds.ToUnSorted(), s.LocalTssData.PartyID),
	)

	s.Logger.Infof("partyIds {%+v}, local partyId index {%d}", s.LocalTssData.PartyIds, s.LocalTssData.PartyID.Index)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(messageBytes[:]))

	turnDuration := rosenTss.GetConfig().TurnDuration
	round := time.Now().Unix() / turnDuration

	//use old setup message if exist
	var setupSignMessage models.SetupSign
	if s.SetupSignMessage.Hash != "" {
		setupSignMessage = s.SetupSignMessage
	} else {
		setupSignMessage = models.SetupSign{
			Hash:      utils.Encoder(messageBytes[:]),
			Peers:     s.LocalTssData.PartyIds,
			Timestamp: round,
			StarterId: s.LocalTssData.PartyID,
		}
	}

	marshal, err := json.Marshal(setupSignMessage)
	if err != nil {
		return err
	}

	payload := models.Payload{
		Message:   string(marshal),
		MessageId: messageId,
		SenderId:  s.LocalTssData.PartyID.Id,
		Name:      setupMessage,
	}
	err = s.NewMessage(rosenTss, payload, "")
	if err != nil {
		return err
	}
	return nil
}

func (s *OperationSign) NewMessage(rosenTss _interface.RosenTss, payload models.Payload, receiver string) error {

	ks, sharedId := s.GetData()
	index := utils.IndexOf(ks, sharedId)
	if index == -1 {
		return fmt.Errorf("index not founded")
	}
	marshal, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	signature, err := s.Sign(marshal)
	if err != nil {
		return err
	}
	gossipMessage := models.GossipMessage{
		Message:    payload.Message,
		MessageId:  payload.MessageId,
		SenderId:   payload.SenderId,
		ReceiverId: receiver,
		Name:       payload.Name,
		Signature:  signature,
		Index:      index,
	}
	err = rosenTss.GetConnection().Publish(gossipMessage)
	if err != nil {
		return err
	}
	return nil
}

// SetupMessageHandler handles setup message
func (s *OperationSign) SetupMessageHandler(
	rosenTss _interface.RosenTss, gossipMessage models.GossipMessage,
	ks []*big.Int,
) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {

		setupSignMessage := &models.SetupSign{}
		err := json.Unmarshal([]byte(gossipMessage.Message), setupSignMessage)
		if err != nil {
			return err
		}

		s.Logger.Infof("setup Message data: %+v", setupSignMessage)

		turnDuration := rosenTss.GetConfig().TurnDuration
		round := time.Now().Unix() / turnDuration

		if round%int64(len(ks)) != int64(gossipMessage.Index) {
			s.Logger.Errorf("it's not %s turn.", gossipMessage.SenderId)
			return nil
		}

		msgBytes, _ := utils.Decoder(s.SignMessage.Message)
		signData := new(big.Int).SetBytes(msgBytes)
		signDataBytes := blake2b.Sum256(signData.Bytes())
		if s.SetupSignMessage.Hash != utils.Encoder(signDataBytes[:]) {
			return fmt.Errorf(
				"wrogn hash to sign\nreceivedHash: %s\nexpectedHash: %s", s.SetupSignMessage.Hash,
				utils.Encoder(signDataBytes[:]),
			)
		}

		for _, peer := range setupSignMessage.Peers {
			if !utils.IsPartyExist(peer, s.LocalTssData.PartyIds) {
				err := s.NewRegister(rosenTss, peer.Id)
				if err != nil {
					return err
				}
			}
		}
		s.SetupSignMessage = *setupSignMessage
	}
	return nil
}

func (s *OperationSign) SignStarter(rosenTss _interface.RosenTss, senderId string) error {
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())

	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(signDataBytes[:]))

	payload := models.Payload{
		Message:   s.SetupSignMessage.Hash,
		MessageId: messageId,
		SenderId:  s.LocalTssData.PartyID.Id,
		Name:      signMessage,
	}
	err := s.NewMessage(rosenTss, payload, senderId)
	if err != nil {
		return err
	}
	s.SetupSignMessage = models.SetupSign{}
	return nil
}

func (s *OperationSign) SignStarterThread(rosenTss _interface.RosenTss) error {
	ticker := time.NewTicker(time.Second * time.Duration(rosenTss.GetConfig().SignStartTimeTracker))
	done := make(chan bool)

	for {
		select {
		case <-done:
			return nil
		case <-ticker.C:
			if s.SetupSignMessage.Hash != "" {
				allPeersExist := true
				for _, peer := range s.SetupSignMessage.Peers {
					if !utils.IsPartyExist(peer, s.LocalTssData.PartyIds) {
						allPeersExist = false
					}
				}
				if allPeersExist {
					err := s.SignStarter(rosenTss, s.SetupSignMessage.StarterId.Id)
					if err != nil {
						return err
					}
					ticker.Stop()
					done <- true
				}
			}
		}
	}
}

func (s *OperationSign) SetupThread(
	rosenTss _interface.RosenTss, ks []*big.Int, shareID *big.Int,
	threshold int,
) error {
	index := int64(utils.IndexOf(ks, shareID))
	length := int64(len(ks))

	turnDuration := rosenTss.GetConfig().TurnDuration
	for {
		round := time.Now().Unix() / turnDuration

		if round%length == index &&
			(time.Now().Unix()-(round*turnDuration)) < rosenTss.GetConfig().LeastProcessRemainingTime {
			if len(s.LocalTssData.PartyIds) >= threshold {
				if s.LocalTssData.Party != nil {
					err := s.Setup(rosenTss)
					if err != nil {
						s.Logger.Errorf("setup function returns error: %+v", err)
						return err
					}
					time.Sleep(time.Second * time.Duration(rosenTss.GetConfig().SetupBroadcastInterval))
				} else {
					return nil
				}
			}
		} else {
			if round%length < index {
				time.Sleep(
					time.Second * time.Duration(
						(index-round%length)*turnDuration-(time.Now().Unix()-(round*turnDuration)),
					),
				)
			} else {
				time.Sleep(
					time.Second * time.Duration(
						(length-round%length+index)*turnDuration-(time.Now().Unix()-(round*turnDuration)),
					),
				)
			}
		}
	}
}

func (s *OperationSign) SignMessageHandler(
	rosenTss _interface.RosenTss,
	gossipMessage models.GossipMessage,
	ks []*big.Int,
	shareID *big.Int,
) error {
	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {

		meta := rosenTss.GetMetaData()

		msgBytes, _ := utils.Decoder(s.SignMessage.Message)
		signData := new(big.Int).SetBytes(msgBytes)
		signDataBytes := blake2b.Sum256(signData.Bytes())
		if gossipMessage.Message != utils.Encoder(signDataBytes[:]) {
			return fmt.Errorf(
				"wrogn hash to sign\nreceivedHash: %s\nexpectedHash: %s", gossipMessage.Message,
				utils.Encoder(signDataBytes[:]),
			)
		}

		s.Signatures[gossipMessage.SenderId] = gossipMessage.Signature
		if len(s.Signatures) >= meta.Threshold {
			messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(signDataBytes[:]))
			payload := models.Payload{
				Message:   gossipMessage.Message,
				MessageId: messageId,
				SenderId:  s.LocalTssData.PartyID.Id,
				Name:      signMessage,
			}
			marshal, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			selfSignature, err := s.Sign(marshal)
			if err != nil {
				return err
			}

			var partyList tss.UnSortedPartyIDs
			for i, peer := range s.LocalTssData.PartyIds {
				_, ok := s.Signatures[peer.Id]
				if !ok {
					partyList =
						append(
							s.LocalTssData.PartyIds[:i].ToUnSorted(),
							s.LocalTssData.PartyIds[i+1:].ToUnSorted()...,
						)
				}
			}

			s.LocalTssData.PartyIds = tss.SortPartyIDs(partyList)

			index := int64(utils.IndexOf(ks, shareID))
			length := int64(len(ks))
			turnDuration := rosenTss.GetConfig().TurnDuration
			round := time.Now().Unix() / turnDuration

			if round%length == index &&
				(time.Now().Unix()-(round*turnDuration)) < rosenTss.GetConfig().LeastProcessRemainingTime {
				s.Signatures[s.LocalTssData.PartyID.Id] = selfSignature
				startSign := models.StartSign{
					Hash:       gossipMessage.Message,
					Signatures: s.Signatures,
					StarterId:  s.LocalTssData.PartyID.Id,
					Peers:      s.LocalTssData.PartyIds,
				}
				marshal, err = json.Marshal(startSign)
				if err != nil {
					return err
				}
				payload = models.Payload{
					Message:   string(marshal),
					MessageId: messageId,
					SenderId:  s.LocalTssData.PartyID.Id,
					Name:      startSignMessage,
				}
				err = s.NewMessage(rosenTss, payload, "")
				if err != nil {
					return nil
				}
			}
		}

	}
	return nil
}
