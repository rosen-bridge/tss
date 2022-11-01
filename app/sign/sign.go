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
	Verify(msg []byte, signature []byte, index int) error
	LoadData(_interface.RosenTss) (*tss.PartyID, error)
	GetData() ([]*big.Int, *big.Int)
	StartParty(
		localTssData *models.TssData,
		peers tss.SortedPartyIDs,
		threshold int,
		signData *big.Int,
		outCh chan tss.Message,
		endCh chan common.SignatureData,
	) error
}

type OperationSign struct {
	_interface.OperationHandler
	LocalTssData         models.TssData
	SignMessage          models.SignMessage
	Signatures           map[int][]byte
	Logger               *zap.SugaredLogger
	SetupSignMessage     models.SetupSign
	SelfSetupSignMessage models.SetupSign
	Handler
}

// Init initializes the eddsa sign partyId and creates partyId message
func (s *OperationSign) Init(rosenTss _interface.RosenTss, receiverId string) error {

	s.Logger.Info("initiation process")

	pID, err := s.LoadData(rosenTss)
	if err != nil {
		s.Logger.Error(err)
		return err
	}
	s.LocalTssData.PartyID = pID
	s.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.LocalTssData.PartyIds.ToUnSorted(), s.LocalTssData.PartyID),
	)

	s.Logger.Infof("local PartyId: %+v", s.LocalTssData.PartyID)
	s.Logger.Info("broadcasting new register message")
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
		keyList, sharedId := s.GetData()
		err := s.SetupThread(rosenTss, keyList, sharedId, meta.Threshold)
		if err != nil {
			errorCh <- err
		}
	}()

	go s.SignStarterThread(rosenTss)

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
			s.Logger.Infof("new {%s} message on communication channel", msg.Name)
			payload := models.Payload{
				Message:   msg.Message,
				MessageId: msg.MessageId,
				SenderId:  msg.SenderId,
				Name:      msg.Name,
			}
			marshal, err := json.Marshal(payload)
			if err != nil {
				s.Logger.Warn(err)
				continue
			}
			err = s.Verify(marshal, msg.Signature, msg.Index)
			if err != nil {
				s.Logger.Warnf("can not verify the message: %v", msg.Name)
				continue
			}

			switch msg.Name {
			case registerMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					err := s.RegisterMessageHandler(rosenTss, msg)
					if err != nil {
						s.Logger.Error(err)
						continue
					}
				}
			case setupMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					keyList, _ := s.GetData()
					err := s.SetupMessageHandler(rosenTss, msg, keyList)
					if err != nil {
						s.Logger.Error(err)
						continue
					}
				}
			case signMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					keyList, sharedId := s.GetData()
					err := s.SignMessageHandler(rosenTss, msg, keyList, sharedId, errorCh)
					if err != nil {
						s.Logger.Error(err)
						continue
					}
				}
			case partyMessage:
				s.Logger.Infof("received party message from: %s", msg.SenderId)
				msgBytes, err := utils.Decoder(msg.Message)
				if err != nil {
					return err
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					return err
				}
				for {
					if s.LocalTssData.Party == nil {
						time.Sleep(100 * time.Millisecond)
					} else {
						break
					}
				}
				s.Logger.Infof("party: %+v", s.LocalTssData.Party)
				err = s.PartyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case startSignMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					keyList, _ := s.GetData()
					peers, err := s.StartSignMessageHandler(rosenTss, msg, keyList)
					if err != nil {
						s.Logger.Error(err)
						continue
					}
					s.CreateParty(rosenTss, peers, errorCh)
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

	s.Logger.Info("creating register message and sending to p2p")

	var noAnswer bool

	if receiverId != "" {
		for _, partyId := range s.LocalTssData.PartyIds {
			if partyId.Id == receiverId {
				noAnswer = true
			}
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
		Message:   utils.Encoder(marshal),
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

		registerMsg := &models.Register{}
		decodedMessage, err := utils.Decoder(gossipMessage.Message)
		if err != nil {
			return err
		}
		err = json.Unmarshal(decodedMessage, registerMsg)
		if err != nil {
			return err
		}

		s.Logger.Infof("new register message: %+v", registerMsg)

		key, _ := new(big.Int).SetString(registerMsg.Key, 10)
		newParty := tss.NewPartyID(registerMsg.Id, registerMsg.Moniker, key)

		if !registerMsg.NoAnswer {
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

		s.Logger.Infof("localparties: %+v", s.LocalTssData.PartyIds)
	}
	return nil
}

// Setup called after if Init up was successful. it used to create party params and sign message
func (s *OperationSign) Setup(rosenTss _interface.RosenTss) error {
	s.Logger.Infof("starting setup process")
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(messageBytes[:]))

	turnDuration := rosenTss.GetConfig().TurnDuration
	round := time.Now().Unix() / turnDuration

	//use old setup message if exist
	var setupSignMessage models.SetupSign
	if s.SelfSetupSignMessage.Hash != "" {
		setupSignMessage = s.SelfSetupSignMessage
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
		Message:   utils.Encoder(marshal),
		MessageId: messageId,
		SenderId:  s.LocalTssData.PartyID.Id,
		Name:      setupMessage,
	}
	s.SelfSetupSignMessage = setupSignMessage
	err = s.NewMessage(rosenTss, payload, "")
	if err != nil {
		s.SelfSetupSignMessage = models.SetupSign{}
		return err
	}
	return nil
}

func (s *OperationSign) NewMessage(rosenTss _interface.RosenTss, payload models.Payload, receiver string) error {

	keyList, sharedId := s.GetData()

	index := utils.IndexOf(keyList, sharedId)
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
	keyList []*big.Int,
) error {

	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {

		setupSignMessage := models.SetupSign{}
		decodedMessage, err := utils.Decoder(gossipMessage.Message)
		if err != nil {
			return err
		}
		err = json.Unmarshal(decodedMessage, &setupSignMessage)
		if err != nil {
			return err
		}

		s.Logger.Infof("new setup Message: %+v", setupSignMessage)

		turnDuration := rosenTss.GetConfig().TurnDuration
		round := time.Now().Unix() / turnDuration

		if round%int64(len(keyList)) != int64(gossipMessage.Index) {
			s.Logger.Errorf("it's not %s turn.", gossipMessage.SenderId)
			return nil
		}

		msgBytes, _ := utils.Decoder(s.SignMessage.Message)
		signData := new(big.Int).SetBytes(msgBytes)
		signDataBytes := blake2b.Sum256(signData.Bytes())
		if setupSignMessage.Hash != utils.Encoder(signDataBytes[:]) {
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
		s.SetupSignMessage = setupSignMessage
	}
	return nil
}

func (s *OperationSign) SignStarter(rosenTss _interface.RosenTss) error {
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())

	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(signDataBytes[:]))

	//use sign of hash of setup sign message in payload message
	marshal, err := json.Marshal(s.SetupSignMessage)
	if err != nil {
		return err
	}
	signature, err := s.Sign(marshal)
	if err != nil {
		return err
	}
	payload := models.Payload{
		Message:   utils.Encoder(signature),
		MessageId: messageId,
		SenderId:  s.LocalTssData.PartyID.Id,
		Name:      signMessage,
	}
	err = s.NewMessage(rosenTss, payload, s.SetupSignMessage.StarterId.Id)
	if err != nil {
		return err
	}
	s.SetupSignMessage = models.SetupSign{}
	return nil
}

func (s *OperationSign) SignStarterThread(rosenTss _interface.RosenTss) {
	ticker := time.NewTicker(time.Second * time.Duration(rosenTss.GetConfig().SignStartTimeTracker))
	done := make(chan bool)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if s.LocalTssData.Party != nil {
				ticker.Stop()
				done <- true
				break
			}
			if s.SetupSignMessage.Hash != "" {
				allPeersExist := true
				for _, peer := range s.SetupSignMessage.Peers {
					if !utils.IsPartyExist(peer, s.LocalTssData.PartyIds) {
						allPeersExist = false
					}
				}
				if allPeersExist {
					err := s.SignStarter(rosenTss)
					if err != nil {
						s.Logger.Errorf("there was an error on starting sign: %v", err)
					}
				}
			}
		}
	}
}

func (s *OperationSign) SetupThread(
	rosenTss _interface.RosenTss, keyList []*big.Int, shareID *big.Int,
	threshold int,
) error {
	index := int64(utils.IndexOf(keyList, shareID))
	length := int64(len(keyList))

	turnDuration := rosenTss.GetConfig().TurnDuration

	for {
		round := time.Now().Unix() / turnDuration
		if round%length == index && int64(time.Now().Second()) < rosenTss.GetConfig().LeastProcessRemainingTime {
			if len(s.LocalTssData.PartyIds) > threshold {
				if s.LocalTssData.Party == nil {
					s.Logger.Infof(
						"round, length, round mod ength, index: %d, %d, %d, %d",
						round,
						length,
						round%length,
						index,
					)
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
						(index-round%length)*turnDuration-int64(time.Now().Second()),
					),
				)
			} else {
				time.Sleep(
					time.Second * time.Duration(
						(length-round%length+index)*turnDuration-int64(time.Now().Second()),
					),
				)
			}
		}
	}
}

func (s *OperationSign) SignMessageHandler(
	rosenTss _interface.RosenTss,
	gossipMessage models.GossipMessage,
	keyList []*big.Int,
	shareID *big.Int,
	errorCh chan error,
) error {
	if gossipMessage.SenderId != s.LocalTssData.PartyID.Id {

		meta := rosenTss.GetMetaData()

		msgBytes, _ := utils.Decoder(s.SignMessage.Message)
		signData := new(big.Int).SetBytes(msgBytes)
		signDataBytes := blake2b.Sum256(signData.Bytes())

		// check and verify the sign in the payload message
		marshal, err := json.Marshal(s.SelfSetupSignMessage)
		if err != nil {
			return err
		}
		decodedSign, err := utils.Decoder(gossipMessage.Message)
		if err != nil {
			return err
		}

		s.Logger.Infof("new sign message: %+v", decodedSign)

		err = s.Verify(marshal, decodedSign, gossipMessage.Index)
		if err != nil {
			s.Logger.Errorf(
				"can not verify the sign message, message: %s, sign: %s",
				string(marshal),
				string(decodedSign),
			)
			return err
		}

		// check the sender be in the list peers of setup message
		var peerFounded bool
		for _, peer := range s.SelfSetupSignMessage.Peers {
			if peer.Id == gossipMessage.SenderId {
				peerFounded = true
			}
		}
		if !peerFounded {
			return fmt.Errorf("sender not exist in the list of peers")
		}

		s.Signatures[gossipMessage.Index] = decodedSign
		if len(s.Signatures) >= meta.Threshold {
			messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(signDataBytes[:]))
			index := int64(utils.IndexOf(keyList, shareID))
			length := int64(len(keyList))
			turnDuration := rosenTss.GetConfig().TurnDuration
			round := time.Now().Unix() / turnDuration

			if round%length == index && int64(time.Now().Second()) < rosenTss.GetConfig().LeastProcessRemainingTime {

				//use hash signature data as message
				//timestamp: round added to start sign
				startSign := models.StartSign{
					Hash:       utils.Encoder(signDataBytes[:]),
					Signatures: s.Signatures,
					StarterId:  s.LocalTssData.PartyID,
					Peers:      s.SelfSetupSignMessage.Peers,
					Timestamp:  round,
				}
				marshal, err := json.Marshal(startSign)
				if err != nil {
					return err
				}
				payload := models.Payload{
					Message:   utils.Encoder(marshal),
					MessageId: messageId,
					SenderId:  s.LocalTssData.PartyID.Id,
					Name:      startSignMessage,
				}
				err = s.NewMessage(rosenTss, payload, "")
				if err != nil {
					return err
				}

				// start party
				s.CreateParty(rosenTss, s.SelfSetupSignMessage.Peers, errorCh)
			} else {
				s.SelfSetupSignMessage = models.SetupSign{}
			}
		}
	}
	return nil
}

func (s *OperationSign) StartSignMessageHandler(
	rosenTss _interface.RosenTss,
	msg models.GossipMessage,
	keyList []*big.Int,
) (tss.SortedPartyIDs, error) {

	startSign := models.StartSign{}
	decoder, err := utils.Decoder(msg.Message)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(decoder, &startSign)
	if err != nil {
		return nil, err
	}
	s.Logger.Infof("new start Sign message: %+v", startSign)

	// check to valid the sign process data hash is valid
	// verify with data received at the start of process
	s.Logger.Info("validation startSign hash parameter")
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	hash := utils.Encoder(signDataBytes[:])
	if startSign.Hash != hash {
		return nil, fmt.Errorf("wrogn hash to sign, receivedHash: %s, expectedHash: %s", startSign.Hash, hash)
	}

	turnDuration := rosenTss.GetConfig().TurnDuration
	round := time.Now().Unix() / turnDuration

	if round%int64(len(keyList)) != int64(msg.Index) {
		err = fmt.Errorf("it's not %s turn", msg.SenderId)
		s.Logger.Error(err)
		return nil, err
	}

	s.Logger.Info("verifying every single signature in the startSign message")
	// verify that every signature be valid and every signer be in the peers list
	setupSignMessage := models.SetupSign{
		Hash:      startSign.Hash,
		Peers:     startSign.Peers,
		StarterId: startSign.StarterId,
		Timestamp: startSign.Timestamp,
	}
	marshal, err := json.Marshal(setupSignMessage)
	if err != nil {
		return nil, err
	}

	localPeerExist := utils.IsPartyExist(s.LocalTssData.PartyID, startSign.Peers)
	if !localPeerExist {
		err = fmt.Errorf("this party is not part of signing process")
		s.Logger.Error(err)
		return nil, err
	}

	var validSignsCount int
	for index, signature := range startSign.Signatures {
		var peerFound bool
		for _, peer := range startSign.Peers {
			if keyList[index].Cmp(peer.KeyInt()) == 0 {
				peerFound = true
				break
			}
		}
		if !peerFound {
			s.Logger.Errorf("peer %d not found ", index)
			continue
		}

		err := s.Verify(marshal, signature, index)
		if err == nil {
			validSignsCount += 1
		}
	}

	s.Logger.Info("checking threshold of valid signatures in the startSign")
	// check threshold of peers and signatures
	meta := rosenTss.GetMetaData()
	if validSignsCount < meta.Threshold {
		return nil, fmt.Errorf("not enough signatures to stast signing process")
	}

	return startSign.Peers, nil
}

func (s *OperationSign) CreateParty(rosenTss _interface.RosenTss, peers tss.SortedPartyIDs, errorCh chan error) {
	s.Logger.Info("creating and starting party")
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	outCh := make(chan tss.Message, len(s.LocalTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(s.LocalTssData.PartyIds))

	threshold := rosenTss.GetMetaData().Threshold
	err := s.StartParty(&s.LocalTssData, peers, threshold, signData, outCh, endCh)
	if err != nil {
		errorCh <- err
		return
	}
	s.Logger.Infof("party state: %v ", s.LocalTssData.Party)
	s.Logger.Infof("party state starting: %v", s.LocalTssData.Party.String())
	s.Logger.Infof("party state running: %v", s.LocalTssData.Party.Running())

	go func() {
		result, err := s.GossipMessageHandler(rosenTss, outCh, endCh)
		if err != nil {
			s.Logger.Error(err)
			errorCh <- err
			return
		}
		if result {
			err = fmt.Errorf("close channel")
			s.Logger.Error(err)
			errorCh <- err
			return
		}
	}()
}
