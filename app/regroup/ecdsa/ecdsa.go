package ecdsa

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	ecdsaRegrouping "github.com/binance-chain/tss-lib/ecdsa/resharing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/mr-tron/base58"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/regroup"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"strings"
	"time"
)

type operationECDSARegroup struct {
	operationRegroup regroup.OperationRegroup
	savedData        ecdsaKeygen.LocalPartySaveData
}

var logging *zap.SugaredLogger

func NewRegroupECDSAOperation(regroupMessage models.RegroupMessage) _interface.Operation {
	logging = logger.NewSugar("ecdsa-regroup")

	return &operationECDSARegroup{
		operationRegroup: regroup.OperationRegroup{
			LocalTssData: models.TssRegroupData{
				PeerState: regroupMessage.PeerState,
			},
			RegroupMessage: regroupMessage,
		},
	}
}

// Init initializes the ecdsa regroup partyId and creates partyId message
func (r *operationECDSARegroup) Init(rosenTss _interface.RosenTss, receiverId string) error {
	logging.Info("Init called")

	if r.operationRegroup.LocalTssData.PartyID == nil {
		if r.operationRegroup.LocalTssData.PeerState == 1 {
			var private []byte
			localPriv := rosenTss.GetPrivate("ecdsa")
			if localPriv == "" {
				priv, _, _, err := utils.GenerateECDSAKey()
				if err != nil {
					private = nil
					return err
				}
				err = rosenTss.SetPrivate(models.Private{
					Private: hex.EncodeToString(priv),
					Crypto:  "ecdsa",
				})
				if err != nil {
					return err
				}
				private = priv
			} else {
				priv, err := hex.DecodeString(localPriv)
				if err != nil {
					return err
				}
				private = priv
			}

			key := new(big.Int).SetBytes(private)
			id := xid.New()
			pMoniker := fmt.Sprintf("tssPeer/%s", id.String())
			r.operationRegroup.LocalTssData.PartyID = tss.NewPartyID(id.String(), pMoniker, key)
		}
		if r.operationRegroup.LocalTssData.PeerState == 0 {
			data, pID, err := rosenTss.GetStorage().LoadECDSAKeygen(rosenTss.GetPeerHome())
			if err != nil {
				return err
			}
			if pID == nil {
				return fmt.Errorf("pIDs is nil")

			} else {
				r.savedData = data
				r.operationRegroup.LocalTssData.PartyID = pID
			}
		}
	}

	message := fmt.Sprintf("%s,%s,%d,%s,%d",
		r.operationRegroup.LocalTssData.PartyID.Id, r.operationRegroup.LocalTssData.PartyID.Moniker,
		r.operationRegroup.LocalTssData.PartyID.KeyInt(), "fromRegroup", r.operationRegroup.LocalTssData.PeerState)
	jsonMessage := rosenTss.NewMessage(receiverId, r.operationRegroup.LocalTssData.PartyID.Id, message, "ecdsaRegroup", "partyId")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}

	return nil

}

// Loop listens to the given channel and parsing the message based on the name
func (r *operationECDSARegroup) Loop(rosenTss _interface.RosenTss, messageCh chan models.GossipMessage) error {
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
			switch msg.Name {
			case "partyId":
				if msg.Message != "" {
					err := r.partyIdMessageHandler(rosenTss, msg)
					if err != nil {
						return err
					}
				}
			case "partyMsg":
				logging.Info("received party message:",
					fmt.Sprintf("from: %s", msg.SenderId))
				msgBytes, err := hex.DecodeString(msg.Message)
				if err != nil {
					logging.Error(err)
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					logging.Error(err)
				}
				err = r.partyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case "regroup":
				logging.Info("received regroup message: ",
					fmt.Sprintf("from: %s", msg.SenderId))
				partiesLength := len(r.operationRegroup.LocalTssData.NewPartyIds) + len(r.operationRegroup.LocalTssData.OldPartyIds)
				outCh := make(chan tss.Message, partiesLength)
				endCh := make(chan ecdsaKeygen.LocalPartySaveData, partiesLength)

				for {
					if r.operationRegroup.LocalTssData.RegroupingParams == nil {
						time.Sleep(time.Second)
						continue
					} else {
						break
					}
				}
				if r.operationRegroup.LocalTssData.Party == nil {

					logging.Infof("LocalTssData %+v", r.operationRegroup.LocalTssData)
					r.operationRegroup.LocalTssData.Party = ecdsaRegrouping.NewLocalParty(r.operationRegroup.LocalTssData.RegroupingParams, r.savedData, outCh, endCh)
				}
				if !r.operationRegroup.LocalTssData.Party.Running() {
					go func() {
						if err := r.operationRegroup.LocalTssData.Party.Start(); err != nil {
							logging.Error(err)
							errorCh <- err
							return
						} else {
							logging.Info("party started")
						}
					}()
				}

				go func() {
					result, err := r.gossipMessageHandler(rosenTss, outCh, endCh)
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

// GetClassName returns the class name
func (r *operationECDSARegroup) GetClassName() string {
	return "ecdsaRegroup"
}

// HandleOutMessage handling party messages on out channel
func (r *operationECDSARegroup) handleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message) error {
	msgHex, err := r.operationRegroup.OperationHandler.PartyMessageHandler(partyMsg)
	if err != nil {
		return err
	}

	jsonMessage := rosenTss.NewMessage("", r.operationRegroup.LocalTssData.PartyID.Id, msgHex, "ecdsaRegroup", "partyMsg")
	err = rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}

	meta := models.MetaData{
		PeersCount: len(r.operationRegroup.LocalTssData.NewPartyIds),
		Threshold:  r.operationRegroup.RegroupMessage.NewThreshold,
	}
	err = rosenTss.GetStorage().WriteData(meta, rosenTss.GetPeerHome(), "config.json", "ecdsa")
	if err != nil {
		return err
	}

	return nil
}

// HandleEndMessage handling save data on end cahnnel of party
func (r *operationECDSARegroup) handleEndMessage(rosenTss _interface.RosenTss, saveData ecdsaKeygen.LocalPartySaveData) error {
	if r.operationRegroup.LocalTssData.PeerState == 1 {
		pkX, pkY := saveData.ECDSAPub.X(), saveData.ECDSAPub.Y()
		pk := ecdsa.PublicKey{
			Curve: tss.S256(),
			X:     pkX,
			Y:     pkY,
		}

		public := utils.GetPKFromECDSAPub(pk.X, pk.Y)
		encodedPK := base58.Encode(public)
		logging.Infof("pk length: %d", len(public))
		logging.Infof("base58 pk: %v", encodedPK)

		logging.Infof("reasharing data: %v", saveData.ECDSAPub)

		err := rosenTss.GetStorage().WriteData(saveData, rosenTss.GetPeerHome(), regroup.RegroupFileName, "ecdsa")
		if err != nil {
			return err
		}
	}
	return nil
}

// gossipMessageHandler called in the main loop of message passing between peers for regrouping scenario.
func (r *operationECDSARegroup) gossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan ecdsaKeygen.LocalPartySaveData) (bool, error) {
	for {
		select {
		case partyMsg := <-outCh:
			err := r.handleOutMessage(rosenTss, partyMsg)
			if err != nil {
				return false, err
			}
		case save := <-endCh:
			err := r.handleEndMessage(rosenTss, save)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

// PartyIdMessageHandler handles partyId message and if cals setup functions if patryIds list length was at least equal to the threshold
func (r *operationECDSARegroup) partyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {

	if gossipMessage.SenderId == r.operationRegroup.LocalTssData.PartyID.Id ||
		(gossipMessage.ReceiverId != "" && gossipMessage.ReceiverId != r.operationRegroup.LocalTssData.PartyID.Id) {
		return nil
	}

	logging.Info("received partyId message ",
		fmt.Sprintf("from: %s", gossipMessage.SenderId))
	partyIdParams := strings.Split(gossipMessage.Message, ",")
	key, _ := new(big.Int).SetString(partyIdParams[2], 10)
	newParty := tss.NewPartyID(partyIdParams[0], partyIdParams[1], key)

	switch partyIdParams[3] {

	case "fromRegroup":
		switch partyIdParams[4] {

		case "0":
			if !utils.IsPartyExist(newParty, r.operationRegroup.LocalTssData.OldPartyIds) {
				r.operationRegroup.LocalTssData.OldPartyIds = tss.SortPartyIDs(
					append(r.operationRegroup.LocalTssData.OldPartyIds.ToUnSorted(), newParty))
			}
		case "1":
			if !utils.IsPartyExist(newParty, r.operationRegroup.LocalTssData.NewPartyIds) {
				r.operationRegroup.LocalTssData.NewPartyIds = tss.SortPartyIDs(
					append(r.operationRegroup.LocalTssData.NewPartyIds.ToUnSorted(), newParty))
			}
		}
		allParties := len(r.operationRegroup.LocalTssData.OldPartyIds) + len(r.operationRegroup.LocalTssData.NewPartyIds)
		if r.operationRegroup.LocalTssData.RegroupingParams == nil {
			switch r.operationRegroup.LocalTssData.PeerState {
			case 0:
				if len(r.operationRegroup.LocalTssData.OldPartyIds) >= r.operationRegroup.RegroupMessage.OldThreshold &&
					len(r.operationRegroup.LocalTssData.NewPartyIds) > r.operationRegroup.RegroupMessage.NewThreshold &&
					allParties == (r.operationRegroup.RegroupMessage.PeersCount-1) {
					err := r.setup(rosenTss)
					if err != nil {
						return err
					}
				} else {
					err := r.Init(rosenTss, "")
					if err != nil {
						return err
					}
				}
			case 1:
				if len(r.operationRegroup.LocalTssData.OldPartyIds) > r.operationRegroup.RegroupMessage.OldThreshold &&
					len(r.operationRegroup.LocalTssData.NewPartyIds) >= r.operationRegroup.RegroupMessage.NewThreshold &&
					allParties == (r.operationRegroup.RegroupMessage.PeersCount-1) {
					err := r.setup(rosenTss)
					if err != nil {
						return err
					}
				} else {
					err := r.Init(rosenTss, "")
					if err != nil {
						return err
					}
				}
			}
		}
	default:
		return fmt.Errorf("wrong message")
	}
	return nil
}

// PartyUpdate updates partyIds in ecdsa app party based on received message
func (r *operationECDSARegroup) partyUpdate(partyMsg models.PartyMessage) error {
	dest := partyMsg.To
	if dest == nil {
		err := fmt.Errorf("did not expect a msg to have a nil destination during regrouping")
		logging.Error(err)
		return err
	}

	if partyMsg.IsToOldCommittee || partyMsg.IsToOldAndNewCommittees {
		if r.operationRegroup.LocalTssData.PeerState == 0 {
			for _, destP := range dest {
				if destP.Index == r.operationRegroup.LocalTssData.PartyID.Index {
					err := r.operationRegroup.OperationHandler.SharedPartyUpdater(r.operationRegroup.LocalTssData.Party, partyMsg)
					if err != nil {
						logging.Error(err)
						return err
					}
				}
			}
		}
	}
	if !partyMsg.IsToOldCommittee || partyMsg.IsToOldAndNewCommittees {
		if r.operationRegroup.LocalTssData.PeerState == 1 {
			for _, destP := range dest {
				if destP.Index == r.operationRegroup.LocalTssData.PartyID.Index {
					err := r.operationRegroup.OperationHandler.SharedPartyUpdater(r.operationRegroup.LocalTssData.Party, partyMsg)
					if err != nil {
						logging.Error(err)
						return err
					}
				}
			}
		}
	}

	return nil
}

// Setup called after if Init up was successful. it used to create party params and regroup message
func (r *operationECDSARegroup) setup(rosenTss _interface.RosenTss) error {
	logging.Info("setup called")
	if r.operationRegroup.LocalTssData.PeerState == 0 {
		if !utils.IsPartyExist(r.operationRegroup.LocalTssData.PartyID, r.operationRegroup.LocalTssData.OldPartyIds) {
			r.operationRegroup.LocalTssData.OldPartyIds = tss.SortPartyIDs(
				append(r.operationRegroup.LocalTssData.OldPartyIds.ToUnSorted(), r.operationRegroup.LocalTssData.PartyID))
		}

	} else if r.operationRegroup.LocalTssData.PeerState == 1 {
		if !utils.IsPartyExist(r.operationRegroup.LocalTssData.PartyID, r.operationRegroup.LocalTssData.NewPartyIds) {
			r.operationRegroup.LocalTssData.NewPartyIds = tss.SortPartyIDs(
				append(r.operationRegroup.LocalTssData.NewPartyIds.ToUnSorted(), r.operationRegroup.LocalTssData.PartyID))
		}
	}
	logging.Infof("NewPartyIds: %+v\n", r.operationRegroup.LocalTssData.NewPartyIds)
	logging.Infof("OldPartyIds: %+v\n", r.operationRegroup.LocalTssData.OldPartyIds)

	newCtx := tss.NewPeerContext(r.operationRegroup.LocalTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(r.operationRegroup.LocalTssData.OldPartyIds)
	logging.Info("creating params")

	r.operationRegroup.LocalTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.S256(), oldCtx, newCtx, r.operationRegroup.LocalTssData.PartyID, r.operationRegroup.RegroupMessage.PeersCount, r.operationRegroup.RegroupMessage.OldThreshold,
		len(r.operationRegroup.LocalTssData.NewPartyIds), r.operationRegroup.RegroupMessage.NewThreshold)
	jsonMessage := rosenTss.NewMessage("", r.operationRegroup.LocalTssData.PartyID.Id, "start regroup.", "ecdsaRegroup", "regroup")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	logging.Infof("r.operationRegroup.LocalTssData RegroupingParams: %v\n", *r.operationRegroup.LocalTssData.RegroupingParams)

	return nil
}
