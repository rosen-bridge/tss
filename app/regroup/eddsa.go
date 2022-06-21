package regroup

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaResharing "github.com/binance-chain/tss-lib/eddsa/resharing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/mr-tron/base58"
	"github.com/rs/xid"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"strings"
	"time"
)

type operationEDDSARegroup struct {
	OperationRegroup
	savedData eddsaKeygen.LocalPartySaveData
}

func NewOperationEDDSARegroup(regroupMessage models.RegroupMessage) _interface.Operation {
	return &operationEDDSARegroup{
		OperationRegroup: OperationRegroup{RegroupMessage: regroupMessage},
	}
}

// Init initializes the eddsa sign partyId and creates partyId message
func (r *operationEDDSARegroup) Init(rosenTss _interface.RosenTss, receiverId string) error {
	fmt.Println("Init called")

	if r.LocalTssData.PeerState == 1 {

		var private []byte
		localPriv, err := rosenTss.GetPrivate("eddsa")
		if err != nil {
			return err
		}
		if localPriv == "" {
			priv, _, _, err := utils.GenerateEDDSAKey()
			if err != nil {
				private = nil
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
		r.LocalTssData.PartyID = tss.NewPartyID(r.LocalTssData.PartyID.Id, pMoniker, key)
	}
	if r.LocalTssData.PeerState == 0 || r.LocalTssData.PeerState == 2 {
		data, pID, err := rosenTss.GetStorage().LoadEDDSAKeygen(rosenTss.GetPeerHome())
		if err != nil {
			return err
		}
		if pID == nil {
			return fmt.Errorf("pIDs is nil")

		} else {
			r.savedData = data
			r.LocalTssData.PartyID = pID
		}
	}

	message := fmt.Sprintf("%s,%s,%d,%s,%d",
		r.LocalTssData.PartyID.Id, r.LocalTssData.PartyID.Moniker,
		r.LocalTssData.PartyID.KeyInt(), "fromRegroup", r.LocalTssData.PeerState)
	jsonMessage := rosenTss.NewMessage(receiverId, r.LocalTssData.PartyID.Id, message, "regroup", "partyId")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil

}

// Loop listens to the given channel and parsing the message based on the name
func (r *operationEDDSARegroup) Loop(rosenTss _interface.RosenTss, messageCh chan models.Message) error {
	models.Logger.Infof("channel", messageCh)
	//msgBytes, _ := hex.DecodeString(r.SignMessage.Message)
	//signData := new(big.Int).SetBytes(msgBytes)

	for {
		select {
		case message, ok := <-messageCh:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			msg := message.Message
			models.Logger.Infof("msg.name: {%s}", msg.Name)
			switch msg.Name {
			case "partyId":
				if msg.Message != "" {
					err := r.partyIdMessageHandler(rosenTss, msg)
					if err != nil {
						return err
					}
				}
			case "partyMsg":
				models.Logger.Info("received party message:",
					fmt.Sprintf("from: %s", msg.SenderId))
				msgBytes, err := hex.DecodeString(msg.Message)
				if err != nil {
					models.Logger.Error(err)
				}
				partyMsg := models.PartyMessage{}
				err = json.Unmarshal(msgBytes, &partyMsg)
				if err != nil {
					models.Logger.Error(err)
				}
				err = r.partyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case "regroup":
				models.Logger.Info("received regroup message: ",
					fmt.Sprintf("from: %s", msg.SenderId))
				partiesLength := len(r.LocalTssData.NewPartyIds) + len(r.LocalTssData.OldPartyIds)
				outCh := make(chan tss.Message, partiesLength)
				endCh := make(chan eddsaKeygen.LocalPartySaveData, partiesLength)

				for {
					if r.LocalTssData.RegroupingParams == nil {
						time.Sleep(time.Second)
						continue
					} else {
						break
					}
				}

				if r.LocalTssData.Party == nil {
					models.Logger.Info(r.LocalTssData)
					if r.LocalTssData.PeerState == 0 || r.LocalTssData.PeerState == 2 {
						r.LocalTssData.Party = eddsaResharing.NewLocalParty(r.LocalTssData.RegroupingParams, r.savedData, outCh, endCh)

					} else if r.LocalTssData.PeerState == 1 {
						save := eddsaKeygen.NewLocalPartySaveData(len(r.LocalTssData.NewPartyIds))
						r.LocalTssData.Party = eddsaResharing.NewLocalParty(r.LocalTssData.RegroupingParams, save, outCh, endCh)
					}
					if err := r.LocalTssData.Party.Start(); err != nil {
						return err
					} else {
						models.Logger.Info("party started")
					}
					go func() {
						err := r.gossipMessageHandler(rosenTss, outCh, endCh)
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

// GetClassName returns the class name
func (r *operationEDDSARegroup) GetClassName() string {
	return "eddsaRegroup"
}

// PartyIdMessageHandler handles partyId message and if cals setup functions if patryIds list length was at least equal to the threshold
func (r *operationEDDSARegroup) partyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {
	if gossipMessage.SenderId != r.LocalTssData.PartyID.Id &&
		(gossipMessage.ReceiverId == "" || gossipMessage.ReceiverId == r.LocalTssData.PartyID.Id) {

		models.Logger.Info("received partyId message ",
			fmt.Sprintf("from: %s", gossipMessage.SenderId))
		partyIdParams := strings.Split(gossipMessage.Message, ",")
		models.Logger.Infof("partyIdParams: %v", partyIdParams)
		key, _ := new(big.Int).SetString(partyIdParams[2], 10)
		newParty := tss.NewPartyID(partyIdParams[0], partyIdParams[1], key)

		meta := rosenTss.GetMetaData()

		switch partyIdParams[3] {

		case "fromRegroup":
			switch partyIdParams[3] {

			case "0":
				if !utils.IsPartyExist(newParty, r.LocalTssData.OldPartyIds) {
					r.LocalTssData.OldPartyIds = tss.SortPartyIDs(
						append(r.LocalTssData.OldPartyIds.ToUnSorted(), newParty))
				}
			case "1":
				if !utils.IsPartyExist(newParty, r.LocalTssData.NewPartyIds) {
					r.LocalTssData.NewPartyIds = tss.SortPartyIDs(
						append(r.LocalTssData.NewPartyIds.ToUnSorted(), newParty))
				}
			case "2":
				if !utils.IsPartyExist(newParty, r.LocalTssData.NewPartyIds) {
					r.LocalTssData.NewPartyIds = tss.SortPartyIDs(
						append(r.LocalTssData.NewPartyIds.ToUnSorted(), newParty))
				}
				if !utils.IsPartyExist(newParty, r.LocalTssData.OldPartyIds) {
					r.LocalTssData.OldPartyIds = tss.SortPartyIDs(
						append(r.LocalTssData.OldPartyIds.ToUnSorted(), newParty))
				}
			}

			lenAllParties := len(r.LocalTssData.OldPartyIds) + len(r.LocalTssData.NewPartyIds)
			if len(r.LocalTssData.OldPartyIds) < meta.Threshold || lenAllParties < r.RegroupMessage.NewThreshold {
				err := r.Init(rosenTss, newParty.Id)
				if err != nil {
					return err
				}
			} else {
				err := r.setup(rosenTss)
				if err != nil {
					return err
				}
			}

		default:
			return fmt.Errorf("wrong message")
		}
	}
	return nil
}

// PartyUpdate updates partyIds in eddsa app party based on received message
func (r *operationEDDSARegroup) partyUpdate(partyMsg models.PartyMessage) error {
	panic("implement me")
}

// Setup called after if Init up was successful. it used to create party params and sign message
func (r *operationEDDSARegroup) setup(rosenTss _interface.RosenTss) error {
	models.Logger.Info("regroup called")
	models.Logger.Infof("len NewPartyIds: %d", len(r.LocalTssData.NewPartyIds))
	models.Logger.Infof("len OldPartyIds: %d", len(r.LocalTssData.OldPartyIds))

	if r.LocalTssData.PeerState == 0 {
		r.LocalTssData.OldPartyIds = tss.SortPartyIDs(
			append(r.LocalTssData.OldPartyIds.ToUnSorted(), r.LocalTssData.PartyID))

	} else if r.LocalTssData.PeerState == 1 {
		r.LocalTssData.NewPartyIds = tss.SortPartyIDs(
			append(r.LocalTssData.NewPartyIds.ToUnSorted(), r.LocalTssData.PartyID))

	}
	if r.LocalTssData.PeerState == 2 {
		r.LocalTssData.OldPartyIds = tss.SortPartyIDs(
			append(r.LocalTssData.OldPartyIds.ToUnSorted(), r.LocalTssData.PartyID))

		r.LocalTssData.NewPartyIds = tss.SortPartyIDs(
			append(r.LocalTssData.NewPartyIds.ToUnSorted(), r.LocalTssData.PartyID))
	}

	newCtx := tss.NewPeerContext(r.LocalTssData.NewPartyIds)
	oldCtx := tss.NewPeerContext(r.LocalTssData.OldPartyIds)
	models.Logger.Info("creating params")

	r.LocalTssData.RegroupingParams = tss.NewReSharingParameters(
		tss.Edwards(), oldCtx, newCtx, r.LocalTssData.PartyID, r.RegroupMessage.PeersCount, r.RegroupMessage.OldThreshold,
		len(r.LocalTssData.NewPartyIds), r.RegroupMessage.NewThreshold)

	models.Logger.Infof("r.LocalTssData RegroupingParams: %v\n", *r.LocalTssData.RegroupingParams)

	//m := models.GossipMessage{
	//	Name:     "regroup",
	//	Message:  "regroup pre params creates.",
	//	SenderId: r.LocalTssData.PartyID.Id,
	//}
	//models.Logger.Infof("regroup message: %v\n", m)
	//msgBytes, err := json.Marshal(m)
	//if err != nil {
	//	panic(err)
	//}
	//sendCh <- msgBytes
	return nil
}

// gossipMessageHandler called in the main loop of message passing between peers for signing scenario.
func (r *operationEDDSARegroup) gossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan eddsaKeygen.LocalPartySaveData) error {
	for {
		select {
		case partyMsg := <-outCh:
			msgHex, err := r.PartyMessageHandler(partyMsg)
			if err != nil {
				return err
			}

			jsonMessage := rosenTss.NewMessage("", r.LocalTssData.PartyID.Id, msgHex, "regroup", "partyMsg")
			err = rosenTss.GetConnection().Publish(jsonMessage)
			if err != nil {
				return err
			}
		case save := <-endCh:

			err := r.saveEDDSARegroupData(rosenTss, save)
			if err != nil {
				return err
			}

		}
	}
}

func (r *operationEDDSARegroup) saveEDDSARegroupData(rosenTss _interface.RosenTss, save eddsaKeygen.LocalPartySaveData) error {
	if r.LocalTssData.PeerState == 1 || r.LocalTssData.PeerState == 2 {
		pkX, pkY := save.EDDSAPub.X(), save.EDDSAPub.Y()
		pk := edwards.PublicKey{
			Curve: tss.Edwards(),
			X:     pkX,
			Y:     pkY,
		}

		public := utils.GetPKFromEDDSAPub(pk.X, pk.Y)
		encodedPK := base58.Encode(public)
		models.Logger.Infof("pk length: %d", len(public))
		models.Logger.Infof("base58 pk: %v", encodedPK)

		models.Logger.Infof("reasharing data: %v", save.EDDSAPub)

		err := rosenTss.GetStorage().WriteData(save, rosenTss.GetPeerHome(), regroupFileName, "eddsa")
		if err != nil {
			return err
		}
	}
	return nil
}
