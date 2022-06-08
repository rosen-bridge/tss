package sign

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaSigning "github.com/binance-chain/tss-lib/eddsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"go.uber.org/zap"
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
	name      string
}

func NewSignEDDSAOperation() _interface.Operation {
	return &signEDDSAOperation{
		name: "eddsaSign",
	}
}

func (s *signEDDSAOperation) Init(rosenTss _interface.RosenTss) error {

	models.Logger.Info("setupPeers called")

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
	messageBytes := blake2b.Sum256([]byte(message))
	messageId := hex.EncodeToString(messageBytes[:])
	jsonMessage := rosenTss.NewMessage(s.LocalTssData.PartyID.Id, message, messageId, "partyId")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}

func (s *signEDDSAOperation) Loop(rosenTss _interface.RosenTss, messageCh chan models.Message, signData *big.Int) error {

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
					fmt.Sprintf("from: %s", msg.SenderID))
				msgBytes, err := hex.DecodeString(msg.Message)
				if err != nil {
					models.Logger.Error("failed to decode message", zap.Error(err))
					return err
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					models.Logger.Error(err)
					return err
				}
				err = s.PartyUpdate(partyMsg)
				if err != nil {
					models.Logger.Error(err)
					return err
				}
			case "sign":
				models.Logger.Info("received sign message: ",
					fmt.Sprintf("from: %s", msg.SenderID))
				outCh := make(chan tss.Message, len(s.LocalTssData.Parties))
				endCh := make(chan common.SignatureData, len(s.LocalTssData.Parties))
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
						models.Logger.Errorf("an error occurred while starting the party: {%v}", err)
						return err
					}
					models.Logger.Info("party started")
					go func() {
						err := s.GossipMessageHandler(rosenTss, outCh, endCh, signData)
						if err != nil {
							models.Logger.Error(err)
						}
					}()
				}
			}
		}
	}
}

// PartyIdMessageHandler handles get message from channel and cals initial function
func (s *signEDDSAOperation) PartyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage, signData *big.Int) error {
	models.Logger.Info("received partyId message ",
		fmt.Sprintf("from: %s", gossipMessage.SenderID))
	partyIdParams := strings.Split(gossipMessage.Message, ",")
	models.Logger.Infof("partyIdParams: %v", partyIdParams)
	key, _ := new(big.Int).SetString(partyIdParams[2], 10)
	newParty := tss.NewPartyID(partyIdParams[0], partyIdParams[1], key)

	meta := rosenTss.GetMetaData()

	switch partyIdParams[3] {

	case "fromSign":
		s.LocalTssData.Parties = tss.SortPartyIDs(
			append(s.LocalTssData.Parties.ToUnSorted(), newParty))

		// TODO: send partyId to sender message.
		if len(s.LocalTssData.Parties) < meta.Threshold {
			err := s.Init(rosenTss)
			if err != nil {
				return err
			}
		} else {
			err := s.Setup(rosenTss, signData)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("wrong message")
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
		err := s.sharedPartyUpdater(s.LocalTssData.Party, partyMsg)
		if err != nil {
			models.Logger.Error(err)
			return err
		}

	} else { // point-to-point!
		if dest[0].Index == partyMsg.GetFrom.Index {
			err := fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, partyMsg.GetFrom.Index)
			models.Logger.Error(err)
			return err
		}
		if s.LocalTssData.PartyID.Index == dest[0].Index {
			models.Logger.Infof("updating party state p2p")
			err := s.sharedPartyUpdater(s.LocalTssData.Party, partyMsg)
			if err != nil {
				models.Logger.Error(err)
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
	for {
		if len(s.LocalTssData.Parties) >= meta.Threshold {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}

	s.LocalTssData.Parties = tss.SortPartyIDs(
		append(s.LocalTssData.Parties.ToUnSorted(), s.LocalTssData.PartyID))

	for _, partyId := range s.LocalTssData.Parties {
		if partyId.Id == s.LocalTssData.PartyID.Id {
			s.LocalTssData.PartyID = partyId
			break
		}
	}

	ctx := tss.NewPeerContext(s.LocalTssData.Parties)
	models.Logger.Info("creating params")
	models.Logger.Infof("PartyID: %d, peerId: %s", s.LocalTssData.PartyID.Index, s.LocalTssData.PartyID.Id)

	s.LocalTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, s.LocalTssData.PartyID, len(s.LocalTssData.Parties), meta.Threshold)

	models.Logger.Infof("params created: %v", s.LocalTssData.Params.EC().Params().N)
	models.Logger.Infof("localEDDSAData params: %v\n", *s.LocalTssData.Params)

	messageBytes := blake2b.Sum256(signMsg.Bytes())
	messageId := hex.EncodeToString(messageBytes[:])
	jsonMessage := rosenTss.NewMessage(s.LocalTssData.PartyID.Id, signMsg.String(), messageId, "sign")

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
			msgHex, err := s.signPartyMessageHandler(partyMsg)
			if err != nil {
				models.Logger.Error(err)
				return err
			}
			messageBytes := blake2b.Sum256(signData.Bytes())
			messageId := hex.EncodeToString(messageBytes[:])
			jsonMessage := rosenTss.NewMessage(s.LocalTssData.PartyID.Id, msgHex, messageId, "partyMsg")
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
	return s.name
}
