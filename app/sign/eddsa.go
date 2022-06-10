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

type signEDDSAOperation struct {
	signOperation
	savedData eddsaKeygen.LocalPartySaveData
}

func NewSignEDDSAOperation(signData *big.Int) _interface.Operation {
	return &signEDDSAOperation{
		signOperation: signOperation{signData: signData},
	}
}

func (s *signEDDSAOperation) Init(rosenTss _interface.RosenTss, receiverId string) error {

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
	signDataBytes := blake2b.Sum256(s.signData.Bytes())
	messageId := hex.EncodeToString(signDataBytes[:])
	jsonMessage := rosenTss.NewMessage(receiverId, s.LocalTssData.PartyID.Id, message, messageId, "partyId")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}

func (s *signEDDSAOperation) Loop(rosenTss _interface.RosenTss, messageCh chan models.Message, signData *big.Int) error {

	models.Logger.Infof("channel", messageCh)

	for {
		select {
		case message := <-messageCh:
			msg := message.Message
			models.Logger.Infof("msg.name: {%s}", msg.Name)
			switch msg.Name {
			case "partyId":
				if msg.Message != "" {
					//TODO: resend self partyId to the sender peer
					err := s.PartyIdMessageHandler(rosenTss, msg, signData)
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
				err = s.PartyUpdate(partyMsg)
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
					signMessage, _ := new(big.Int).SetString(msg.Message, 10)
					s.LocalTssData.Party = eddsaSigning.NewLocalParty(signMessage, s.LocalTssData.Params, s.savedData, outCh, endCh)
					if err := s.LocalTssData.Party.Start(); err != nil {
						return err
					}
					models.Logger.Info("party started")
					go func() {
						err := s.GossipMessageHandler(rosenTss, outCh, endCh, signData)
						if err != nil {
							models.Logger.Error(err)
							//TODO: handle error
							return
						}
					}()
				}
			}
		}
	}
}

func isExist(newPartyId *tss.PartyID, partyIds tss.SortedPartyIDs) bool {
	for _, partyId := range partyIds {
		if partyId.Id == newPartyId.Id {
			return true
		}
	}
	return false
}

// PartyIdMessageHandler handles get message from channel and cals initial function
func (s *signEDDSAOperation) PartyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage, signData *big.Int) error {

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
			if !isExist(newParty, s.LocalTssData.PartyIds) {
				s.LocalTssData.PartyIds = tss.SortPartyIDs(
					append(s.LocalTssData.PartyIds.ToUnSorted(), newParty))

				if len(s.LocalTssData.PartyIds) < meta.Threshold {
					err := s.Init(rosenTss, newParty.Id)
					if err != nil {
						return err
					}
				} else {
					err := s.Setup(rosenTss, signData)
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
func (s *signEDDSAOperation) PartyUpdate(partyMsg models.PartyMessage) error {
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

// SetupSign called after if setting up was successful.
// It initializes siging scenario in eddsa. And gathering all parties.
func (s *signEDDSAOperation) Setup(rosenTss _interface.RosenTss, signMsg *big.Int) error {
	meta := rosenTss.GetMetaData()

	models.Logger.Infof("meta %+v", meta)

	if len(s.LocalTssData.PartyIds) < meta.Threshold {
		return fmt.Errorf("not eanough partyId")
	}

	s.LocalTssData.PartyIds = tss.SortPartyIDs(
		append(s.LocalTssData.PartyIds.ToUnSorted(), s.LocalTssData.PartyID))

	models.Logger.Infof("partyIds {%+v}", s.LocalTssData.PartyIds)
	models.Logger.Infof("partyIds count {%+v}", len(s.LocalTssData.PartyIds))
	for _, partyId := range s.LocalTssData.PartyIds {
		models.Logger.Infof("partyId id:%v, index: %v", partyId.Id, partyId.Index)
		if partyId.Id == s.LocalTssData.PartyID.Id {
			s.LocalTssData.PartyID = partyId
			break
		}
	}

	ctx := tss.NewPeerContext(s.LocalTssData.PartyIds)
	models.Logger.Info("creating params")
	models.Logger.Infof("PartyID: %d, peerId: %s", s.LocalTssData.PartyID.Index, s.LocalTssData.PartyID.Id)

	s.LocalTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, s.LocalTssData.PartyID, len(s.LocalTssData.PartyIds), meta.Threshold)

	models.Logger.Infof("params created: %v", s.LocalTssData.Params.EC().Params().N)
	models.Logger.Infof("localEDDSAData params: %v\n", *s.LocalTssData.Params)

	messageBytes := blake2b.Sum256(signMsg.Bytes())
	messageId := hex.EncodeToString(messageBytes[:])
	jsonMessage := rosenTss.NewMessage("", s.LocalTssData.PartyID.Id, signMsg.String(), messageId, "sign")

	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}

// SignMessageHandler called in the main loop of message passing between peers for signing scenario.
// data stored in outCh and endCh in message passing.
func (s *signEDDSAOperation) GossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan common.SignatureData, signData *big.Int) error {
	for {
		select {
		case partyMsg := <-outCh:
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
		case save := <-endCh:

			models.Logger.Infof("sign result: R: {%s}, S: {%s}, M:{%s}\n", hex.EncodeToString(save.R), hex.EncodeToString(save.S), hex.EncodeToString(save.M))
			models.Logger.Infof("signature: %v", save.Signature)
			models.Logger.Info("EDDSA signing test done.")

			messageBytes := blake2b.Sum256(signData.Bytes())
			messageId := hex.EncodeToString(messageBytes[:])
			time.Sleep(2 * time.Second)
			err := rosenTss.GetStorage().WriteData(save, rosenTss.GetPeerHome(), messageId, signFileName, "eddsa")
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func (s *signEDDSAOperation) GetClassName() string {
	return "eddsaSign"
}
