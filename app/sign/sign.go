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

/*	Init
	- Initializes the eddsa sign partyId and creates a broadcast register message by calling NewRegister
	args:
	- app_interface_to_load_data _interface.RosenTss
	- receiver_id string
	returns:
	error
*/
func (s *OperationSign) Init(rosenTss _interface.RosenTss, receiverId string) error {

	s.Logger.Info("initiation signing process")

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
	err = s.NewRegister(rosenTss, "")
	if err != nil {
		s.Logger.Error(err)
		return err
	}
	return nil
}

/*	Loop
	- creates goroutine to handle SetupThread function.
	- creates goroutine to handle SignStarterThread function.
	- reads new gossip messages from channel and handle it by calling related function in a goroutine.
	args:
	- app_interface_to_load_data _interface.RosenTss
	- gossip_meesage_channel chan models.GossipMessage
	returns:
	error
*/
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
				if s.LocalTssData.Party != nil {
					s.Logger.Infof("party was waiting for: %+v", s.LocalTssData.Party.WaitingFor())
				}
				return fmt.Errorf("communication channel is closed")
			}
			s.Logger.Infof("new {%s} message from {%s} on communication channel", msg.Name, msg.SenderId)
			payload := models.Payload{
				Message:   msg.Message,
				MessageId: msg.MessageId,
				SenderId:  msg.SenderId,
				Name:      msg.Name,
			}
			marshal, err := json.Marshal(payload)
			if err != nil {
				s.Logger.Error(err)
				continue
			}
			err = s.Verify(marshal, msg.Signature, msg.Index)
			if err != nil {
				s.Logger.Errorf("can not verify the message: %v", msg.Name)
				continue
			}

			switch msg.Name {
			case registerMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					go func() {
						err := s.RegisterMessageHandler(rosenTss, msg)
						if err != nil {
							s.Logger.Errorf("there was an error in handling register message: %+v", err)
						}
					}()
				}
			case setupMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					keyList, _ := s.GetData()
					go func() {
						err := s.SetupMessageHandler(rosenTss, msg, keyList)
						if err != nil {
							s.Logger.Errorf("there was an error in handling setup message: %+v", err)
						}
					}()
				}
			case signMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					keyList, sharedId := s.GetData()
					go func() {
						err := s.SignMessageHandler(rosenTss, msg, keyList, sharedId, errorCh)
						if err != nil {
							s.Logger.Errorf("there was an error in handling sign message: %+v", err)
						}
					}()
				}
			case partyMessage:
				msgBytes, err := utils.Decoder(msg.Message)
				if err != nil {
					return err
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					return err
				}
				go func() {
					for {
						if s.LocalTssData.Party == nil {
							time.Sleep(time.Duration(rosenTss.GetConfig().WaitInPartyMessageHandling) * time.Millisecond)
						} else {
							break
						}
					}
					s.Logger.Infof("party info: %+v", s.LocalTssData.Party)
					err = s.PartyUpdate(partyMsg)
					if err != nil {
						s.Logger.Errorf("there was an error in handling party message: %+v", err)
						errorCh <- err
					}
					s.Logger.Infof("party is waiting for: %+v", s.LocalTssData.Party.WaitingFor())
				}()
			case startSignMessage:
				if msg.Message != "" && s.LocalTssData.Party == nil {
					keyList, _ := s.GetData()
					peers, err := s.StartSignMessageHandler(rosenTss, msg, keyList)
					if err != nil {
						s.Logger.Errorf("there was an error in handling start sign message: %+v", err)
						s.Logger.Error(err)
						if err.Error() == models.NotPartOfSigningProcess {
							return nil
						}
						return err
					}
					s.CreateParty(rosenTss, peers, errorCh)
					s.Logger.Infof("party is waiting for: %+v", s.LocalTssData.Party.WaitingFor())
				}
			}
		}
	}
}

/*	PartyUpdate
	- Updates party on received message destination.
	args:
	- party_message models.PartyMessage
	returns:
	error
*/
func (s *OperationSign) PartyUpdate(partyMsg models.PartyMessage) error {
	dest := partyMsg.To
	if dest == nil { // broadcast!
		if s.LocalTssData.Party.PartyID().Index == partyMsg.GetFrom.Index {
			return nil
		}
		s.Logger.Infof("updating party state with bradcast message")
		err := s.OperationHandler.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}

	} else { // point-to-point!
		if dest[0].Index == partyMsg.GetFrom.Index {
			err := fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, partyMsg.GetFrom.Index)
			return err
		}
		s.Logger.Infof("updating party state with p2p message")
		err := s.OperationHandler.SharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

/*	HandleOutMessage
	- handles party messages on out channel
	- creates payload from party message
	- send it to NewMessage function
	args:
	- app_interface_to_load_data _interface.RosenTss
	- party_message tss.Message
	returns:
	error
*/
func (s *OperationSign) HandleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message) error {
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	msgHex, err := s.OperationHandler.PartyMessageHandler(partyMsg)
	if err != nil {
		s.Logger.Errorf("there was an error in parsing party message to the struct: %+v", err)
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

/*	HandleEndMessage
	- handles save data (signature) on end channel of party
	- logs the data and send it to CallBack
	args:
	- app_interface_to_load_data _interface.RosenTss
	- signature_data *common.SignatureData
	returns:
	error
*/
func (s *OperationSign) HandleEndMessage(rosenTss _interface.RosenTss, signatureData *common.SignatureData) error {

	signData := models.SignData{
		Signature: utils.Encoder(signatureData.Signature),
		R:         utils.Encoder(signatureData.R),
		S:         utils.Encoder(signatureData.S),
		M:         utils.Encoder(signatureData.M),
	}

	s.Logger.Infof("signing process finished.", s.SignMessage.Crypto)
	s.Logger.Infof("signning result: R: {%s}, S: {%s}, M:{%s}\n", signData.R, signData.S, signData.M)
	s.Logger.Infof("signature: %v", signData.Signature)

	err := rosenTss.GetConnection().CallBack(s.SignMessage.CallBackUrl, signData, "ok")
	if err != nil {
		return err
	}

	return nil

}

/*	GossipMessageHandler
	- handles all party messages on outCh and endCh
	- listens to channels and send the message to the right function
	args:
	- app_interface_to_load_data _interface.RosenTss
	- outCh chan tss.Message
	- endCh chan common.SignatureData
	returns:
	result_of_process bool, error
*/
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

/*	NewRegister
	- creates a payload for register message and sends it to NewMessage function
	args:
	- app_interface_to_load_data _interface.RosenTss
	- receiver_id string
	returns:
	error
*/
func (s *OperationSign) NewRegister(rosenTss _interface.RosenTss, receiverId string) error {

	s.Logger.Infof("creating new register message")

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

/*	RegisterMessageHandler
	- handles register message.
	- unmarshal received message as register message.
	- check NoAnswer parameter to send register message to receiver if needded.
	- add the new partyId to the list if it's not exist.
	args:
	- app_interface_to_load_data _interface.RosenTss
	- gossip_message models.GossipMessage
	returns:
	error
*/
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

		s.Logger.Debugf("new register message: %+v", registerMsg)

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

		s.Logger.Debugf("local party ids list: %+v", s.LocalTssData.PartyIds)
	}
	return nil
}

/*	Setup
	- calculates round and creates setup message.
	- If SelfSetupSignMessage is not nil, creates new setup message from this.
	- creates payload from new setup message and sends it to NewMessage function.
	args:
	- app_interface_to_load_data _interface.RosenTss
	returns:
	error
*/
func (s *OperationSign) Setup(rosenTss _interface.RosenTss) error {
	s.Logger.Infof("starting setup process")
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	messageBytes := blake2b.Sum256(signData.Bytes())
	messageId := fmt.Sprintf("%s%s", s.SignMessage.Crypto, utils.Encoder(messageBytes[:]))

	turnDuration := rosenTss.GetConfig().TurnDuration
	round := time.Now().Unix() / turnDuration

	var peers []tss.PartyID
	for _, peer := range s.LocalTssData.PartyIds {
		peers = append(peers, *peer)
	}
	//use old setup message if exist
	var setupSignMessage models.SetupSign
	if s.SelfSetupSignMessage.Hash != "" {
		setupSignMessage = s.SelfSetupSignMessage
	} else {
		setupSignMessage = models.SetupSign{
			Hash:      utils.Encoder(messageBytes[:]),
			Peers:     peers,
			Timestamp: round,
			StarterId: s.LocalTssData.PartyID.Id,
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

/*	NewMessage
	- finds the index of peer in the key list.
	- creates a gossip message from payload.
	- sends the gossip message to Publish function.
	args:
	- app_interface_to_load_data _interface.RosenTss
	- payload models.Payload
	- receiver_id string
	returns:
	error
*/
func (s *OperationSign) NewMessage(rosenTss _interface.RosenTss, payload models.Payload, receiver string) error {
	s.Logger.Infof("creating new gossip message")
	keyList, sharedId := s.GetData()

	index := utils.IndexOf(keyList, sharedId)
	if index == -1 {
		return fmt.Errorf("party index not found")
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

/*	SetupMessageHandler
	- handles the received setup message
	- calculates round of sender and checks to be it's round.
	- checks the hash in the setup message with sign data hash.
	- register with peers not exist in the peer list
	args:
	- app_interface_to_load_data _interface.RosenTss
	- gossip_message models.GossipMessage
	- key_list []*big.Int,
	returns:
	error
*/
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

		s.Logger.Debugf("new setup Message: %+v", setupSignMessage)

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
				"wrong hash to sign, received: %s, expected: %s", s.SetupSignMessage.Hash,
				utils.Encoder(signDataBytes[:]),
			)
		}

		for _, peer := range setupSignMessage.Peers {
			if !utils.IsPartyExist(&peer, s.LocalTssData.PartyIds) {
				err := s.NewRegister(rosenTss, peer.Id)
				if err != nil {
					return err
				}
			}
		}
		s.SetupSignMessage = setupSignMessage
		s.SelfSetupSignMessage = models.SetupSign{}
	}
	return nil
}

/*	SignStarter
	- signs received setup message
	- sends the signature to the setup message sender
	args:
	- app_interface_to_load_data _interface.RosenTss
	returns:
	error
*/
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
	err = s.NewMessage(rosenTss, payload, s.SetupSignMessage.StarterId)
	if err != nil {
		return err
	}
	s.SetupSignMessage = models.SetupSign{}
	return nil
}

/*	SignStarterThread
	- calls in the Loop function in goroutine
	- creates a time ticker for every SignStartTimeTracker
	- when ticker sends message checks all setup message peers be existed in the peer list
	- if the condition was true it calls the SignStarter function.
	args:
	- app_interface_to_load_data _interface.RosenTss
	returns:
	-
*/
func (s *OperationSign) SignStarterThread(rosenTss _interface.RosenTss) {
	ticker := time.NewTicker(time.Second * time.Duration(rosenTss.GetConfig().SignStartTimeTracker))
	done := make(chan bool)
	go func() {
		for {
			if s.LocalTssData.Party != nil {
				ticker.Stop()
				done <- true
				return
			}
		}
	}()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if s.SetupSignMessage.Hash != "" {
				allPeersExist := true
				for _, peer := range s.SetupSignMessage.Peers {
					if !utils.IsPartyExist(&peer, s.LocalTssData.PartyIds) {
						allPeersExist = false
					}
				}
				if allPeersExist {
					err := s.SignStarter(rosenTss)
					if err != nil {
						s.Logger.Errorf("there was an error on sending sign message: %v", err)
					}
				}
			}
		}
	}
}

/*	SetupThread
	- calculates round of party
	- check to the round be ok, and peer list has more than threshold peer and party be nil
	- if all conditions were ok, then calls Setup function.
	- if conditions were not ok, goes to sleep to be it's turn
	args:
	- app_interface_to_load_data _interface.RosenTss
	- key_list []*big.Int
	- shareID *big.Int
	- threshold int,
	returns:
	error
*/
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
					s.Logger.Debugf(
						"round, length, round mod ength, index: %d, %d, %d, %d",
						round,
						length,
						round%length,
						index,
					)
					err := s.Setup(rosenTss)
					if err != nil {
						s.Logger.Errorf("there was an error on sending setup message to peers: %+v", err)
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

/*	SignMessageHandler
	- handles the received sign message
	- validates senders signatures
	- if the threshold of received signatures was ok, and it was its turn, sends startSign message to other peers.
	- Creates peer party
	args:
	- app_interface_to_load_data _interface.RosenTss
	- gossip_message models.GossipMessage
	- key_list []*big.Int,
	- shareID *big.Int,
	- errorCh chan error,
	returns:
	error
*/
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

		s.Logger.Debugf("new sign message: %+v", decodedSign)

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
					StarterId:  s.LocalTssData.PartyID.Id,
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

/*	StartSignMessageHandler
	- handles the received StartSign message
	- validates the hash data in the message
	- validates to be sender's turn
	- validates to be part of process
	- validates signatures to be valid and more than threshold.
	args:
	- app_interface_to_load_data _interface.RosenTss
	- gossip_message models.GossipMessage
	- key_list []*big.Int,
	returns:
	list_of_peers_to_start_sign_process []tss.PartyID, error
*/
func (s *OperationSign) StartSignMessageHandler(
	rosenTss _interface.RosenTss,
	msg models.GossipMessage,
	keyList []*big.Int,
) ([]tss.PartyID, error) {

	startSign := models.StartSign{}
	decoder, err := utils.Decoder(msg.Message)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(decoder, &startSign)
	if err != nil {
		return nil, err
	}
	s.Logger.Debugf("new start Sign message: %+v", startSign)

	// check to valid the sign process data hash is valid
	// verify with data received at the start of process
	s.Logger.Info("validation startSign hash parameter")
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)
	signDataBytes := blake2b.Sum256(signData.Bytes())
	hash := utils.Encoder(signDataBytes[:])
	if startSign.Hash != hash {
		return nil, fmt.Errorf("wrong hash to sign, receivedHash: %s, expectedHash: %s", startSign.Hash, hash)
	}

	turnDuration := rosenTss.GetConfig().TurnDuration
	round := time.Now().Unix() / turnDuration

	if round%int64(len(keyList)) != int64(msg.Index) {
		err = fmt.Errorf("it's not %s turn", msg.SenderId)
		s.Logger.Error(err)
		return nil, err
	}

	s.Logger.Info("verifying signatures in the start sign message")
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

	var localPeerExist bool
	for _, peer := range startSign.Peers {
		if s.LocalTssData.PartyID.Id == peer.Id {
			localPeerExist = true
		}
	}
	if !localPeerExist {
		return nil, fmt.Errorf(models.NotPartOfSigningProcess)
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

/*	CreateParty
	- creates end and out channel for party,
	- calls StartParty function of protocol
	- handles end channel and out channel in a goroutine
	args:
	- app_interface_to_load_data _interface.RosenTss
	- peers []tss.PartyID
	- errorCh chan error
	returns:
	-
*/
func (s *OperationSign) CreateParty(rosenTss _interface.RosenTss, peers []tss.PartyID, errorCh chan error) {
	s.Logger.Info("creating and starting party")
	msgBytes, _ := utils.Decoder(s.SignMessage.Message)
	signData := new(big.Int).SetBytes(msgBytes)

	outCh := make(chan tss.Message, len(s.LocalTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(s.LocalTssData.PartyIds))

	threshold := rosenTss.GetMetaData().Threshold
	var unsortedPeers []*tss.PartyID
	for _, peer := range peers {
		unsortedPeers = append(unsortedPeers, tss.NewPartyID(peer.Id, peer.Moniker, peer.KeyInt()))
	}
	err := s.StartParty(&s.LocalTssData, tss.SortPartyIDs(unsortedPeers), threshold, signData, outCh, endCh)
	if err != nil {
		s.Logger.Errorf("there was an error in starting party: %+v", err)
		errorCh <- err
		return
	}

	s.Logger.Infof("party info: %v ", s.LocalTssData.Party)
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
