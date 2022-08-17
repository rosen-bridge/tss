package ecdsa

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/mr-tron/base58"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/keygen"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
	"strings"
	"time"
)

type operationECDSAKeygen struct {
	keygen.OperationKeygen
}

var logging *zap.SugaredLogger

func NewKeygenECDSAOperation(keygenMessage models.KeygenMessage) _interface.Operation {
	logging = logger.NewSugar("ecdsa-keygen")
	return &operationECDSAKeygen{
		keygen.OperationKeygen{
			KeygenMessage: keygenMessage,
		},
	}
}

func (k *operationECDSAKeygen) Init(rosenTss _interface.RosenTss, receiverId string) error {
	logging.Info("Init called")

	if k.LocalTssData.PartyID == nil {
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

		id := xid.New()
		key := new(big.Int).SetBytes(private)
		pMoniker := fmt.Sprintf("tssPeer/%s", id.String())
		k.LocalTssData.PartyID = tss.NewPartyID(id.String(), pMoniker, key)
	}
	message := fmt.Sprintf("%s,%s,%d,%s",
		k.LocalTssData.PartyID.Id, k.LocalTssData.PartyID.Moniker,
		k.LocalTssData.PartyID.KeyInt(), "fromKeygen")
	jsonMessage := rosenTss.NewMessage(receiverId, k.LocalTssData.PartyID.Id, message, "ecdsaKeygen", "partyId")
	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}

// Loop listens to the given channel and parsing the message based on the name
func (k *operationECDSAKeygen) Loop(rosenTss _interface.RosenTss, messageCh chan models.GossipMessage) error {
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
					err := k.partyIdMessageHandler(rosenTss, msg)
					if err != nil {
						return err
					}
				}
			case "partyMsg":
				logging.Info("received party message:",
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
				err = k.partyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case "keygen":
				logging.Info("received keygen message: ",
					fmt.Sprintf("from: %s", msg.SenderId))
				outCh := make(chan tss.Message, len(k.LocalTssData.PartyIds))
				endCh := make(chan ecdsaKeygen.LocalPartySaveData, len(k.LocalTssData.PartyIds))
				for {
					if k.LocalTssData.Params == nil {
						time.Sleep(time.Second)
					} else {
						break
					}
				}

				if k.LocalTssData.Party == nil {
					k.LocalTssData.Party = ecdsaKeygen.NewLocalParty(k.LocalTssData.Params, outCh, endCh)
				}
				if !k.LocalTssData.Party.Running() {
					go func() {
						if err := k.LocalTssData.Party.Start(); err != nil {
							logging.Error(err)
							errorCh <- err
							return
						}
						logging.Info("party started")
					}()
					go func() {
						result, err := k.gossipMessageHandler(rosenTss, outCh, endCh)
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
func (k *operationECDSAKeygen) GetClassName() string {
	return "ecdsaKeygen"
}

// HandleOutMessage handling party messages on out channel
func (k *operationECDSAKeygen) handleOutMessage(rosenTss _interface.RosenTss, partyMsg tss.Message) error {
	msgHex, err := k.PartyMessageHandler(partyMsg)
	if err != nil {
		return err
	}

	jsonMessage := rosenTss.NewMessage("", k.LocalTssData.PartyID.Id, msgHex, "ecdsaKeygen", "partyMsg")
	err = rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil

}

// HandleEndMessage handling save data on end cahnnel of party
func (k *operationECDSAKeygen) handleEndMessage(rosenTss _interface.RosenTss, saveData ecdsaKeygen.LocalPartySaveData) error {
	index, err := saveData.OriginalIndex()
	if err != nil {
		return fmt.Errorf("should not be an error getting a party's index from save data %v", err)
	}
	logging.Infof("data index %v", index)

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

	err = rosenTss.GetStorage().WriteData(saveData, rosenTss.GetPeerHome(), keygen.KeygenFileName, "ecdsa")
	if err != nil {
		return err
	}

	data := struct {
		PeersCount int    `json:"peersCount"`
		Threshold  int    `json:"threshold"`
		Crypto     string `json:"crypto"`
		PubKey     string `json:"pubKey"`
	}{
		PeersCount: k.OperationKeygen.KeygenMessage.PeersCount,
		Threshold:  k.OperationKeygen.KeygenMessage.Threshold,
		Crypto:     k.OperationKeygen.KeygenMessage.Crypto,
		PubKey:     encodedPK,
	}

	err = rosenTss.GetConnection().CallBack(k.OperationKeygen.KeygenMessage.CallBackUrl, data, "ok")
	if err != nil {
		return err
	}

	return nil
}

// GossipMessageHandler handling all party messages on outCH and endCh
func (k *operationECDSAKeygen) gossipMessageHandler(rosenTss _interface.RosenTss, outCh chan tss.Message, endCh chan ecdsaKeygen.LocalPartySaveData) (bool, error) {
	for {
		select {
		case partyMsg := <-outCh:
			err := k.handleOutMessage(rosenTss, partyMsg)
			if err != nil {
				return false, err
			}
		case save := <-endCh:
			err := k.handleEndMessage(rosenTss, save)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

// PartyIdMessageHandler handles partyId message and if cals setup functions if patryIds list length was at least equal to the threshold
func (k *operationECDSAKeygen) partyIdMessageHandler(rosenTss _interface.RosenTss, gossipMessage models.GossipMessage) error {

	if gossipMessage.SenderId != k.LocalTssData.PartyID.Id &&
		(gossipMessage.ReceiverId == "" || gossipMessage.ReceiverId == k.LocalTssData.PartyID.Id) {

		logging.Info("received partyId message ",
			fmt.Sprintf("from: %s", gossipMessage.SenderId))
		partyIdParams := strings.Split(gossipMessage.Message, ",")
		key, _ := new(big.Int).SetString(partyIdParams[2], 10)
		newParty := tss.NewPartyID(partyIdParams[0], partyIdParams[1], key)

		meta := rosenTss.GetMetaData()

		switch partyIdParams[3] {

		case "fromKeygen":
			if !utils.IsPartyExist(newParty, k.LocalTssData.PartyIds) {
				k.LocalTssData.PartyIds = tss.SortPartyIDs(
					append(k.LocalTssData.PartyIds.ToUnSorted(), newParty))
			}
			if k.LocalTssData.Params == nil {
				if len(k.LocalTssData.PartyIds) >= meta.Threshold {
					err := k.setup(rosenTss)
					if err != nil {
						return err
					}
				} else {
					err := k.Init(rosenTss, "")
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
func (k *operationECDSAKeygen) partyUpdate(partyMsg models.PartyMessage) error {
	dest := partyMsg.To
	if dest == nil { // broadcast!

		if k.LocalTssData.Party.PartyID().Index == partyMsg.GetFrom.Index {
			return nil
		}
		logging.Infof("updating party state")
		err := k.SharedPartyUpdater(k.LocalTssData.Party, partyMsg)
		if err != nil {
			return err
		}

	} else { // point-to-point!
		if dest[0].Index == partyMsg.GetFrom.Index {
			err := fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, partyMsg.GetFrom.Index)
			return err
		}
		if k.LocalTssData.PartyID.Index == dest[0].Index {
			logging.Infof("updating party state p2p")
			err := k.SharedPartyUpdater(k.LocalTssData.Party, partyMsg)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

// Setup called after if Init up was successful. it used to create party params and keygen message
func (k *operationECDSAKeygen) setup(rosenTss _interface.RosenTss) error {
	meta := rosenTss.GetMetaData()

	logging.Infof("meta %+v", meta)

	if len(k.LocalTssData.PartyIds) < meta.Threshold {
		return fmt.Errorf("not eanough partyId")
	}

	logging.Info("setup tss called")

	k.LocalTssData.PartyIds = tss.SortPartyIDs(append(k.LocalTssData.PartyIds.ToUnSorted(), k.LocalTssData.PartyID))

	ctx := tss.NewPeerContext(k.LocalTssData.PartyIds)
	for _, id := range k.LocalTssData.PartyIds {
		logging.Infof("PartyID: %v	, peerId: %s, key: %v", id, id.Id, id.KeyInt())
		if id.Id == k.LocalTssData.PartyID.Id {
			k.LocalTssData.PartyID = id
		}
	}
	logging.Infof("PartyID: %d, peerId: %s", k.LocalTssData.PartyID.Index, k.LocalTssData.PartyID.Id)

	logging.Info("creating params")
	k.LocalTssData.Params = tss.NewParameters(
		tss.S256(), ctx, k.LocalTssData.PartyID, len(k.LocalTssData.PartyIds), meta.Threshold)

	jsonMessage := rosenTss.NewMessage("", k.LocalTssData.PartyID.Id, "generate key", "ecdsaKeygen", "keygen")

	err := rosenTss.GetConnection().Publish(jsonMessage)
	if err != nil {
		return err
	}
	return nil
}
