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
	logging = logger.NewSugar(signMessage.Crypto + "-sign")
	return &operationECDSASign{
		operationSign: sign.OperationSign{
			SignMessage: signMessage,
			PeersMap:    make(map[string]string),
			Signatures:  make(map[string]string),
			Logger:      logging,
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

	err := s.operationSign.NewRegister(rosenTss, receiverId, s.signMessage)
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

	go func() {
		index := int64(utils.IndexOf(s.savedData.Ks, s.savedData.ShareID))
		length := int64(len(s.savedData.Ks))
		for {
			minutes := time.Now().Unix() / 60
			if minutes%length == index && (time.Now().Unix()-(minutes*60)) < 50 {
				if !utils.IsPartyExist(s.operationSign.LocalTssData.PartyID, s.operationSign.LocalTssData.PartyIds) {
					err := s.operationSign.Setup(rosenTss, s.signMessage)
					if err != nil {
						logging.Errorf("setup function returns error: %+v", err)
						errorCh <- err
					}
				}
				return
			} else {
				time.Sleep(time.Second * time.Duration((length-minutes%length)*60-(time.Now().Unix()-(minutes*60))))
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		done := make(chan bool)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if s.operationSign.SetupSignMessage.Hash != "" {
					state := true
					for _, peer := range s.operationSign.SetupSignMessage.Peers {
						if !utils.IsPartyExist(peer, s.operationSign.LocalTssData.PartyIds) {
							state = false
						}
					}
					if state {
						starterP2pId, ok := s.operationSign.PeersMap[s.operationSign.SetupSignMessage.StarterId.Id]
						if !ok {
							err := fmt.Errorf("peers with this id: %s, not founded", s.operationSign.SetupSignMessage.StarterId)
							errorCh <- err
						}
						err := s.operationSign.SignStarter(rosenTss, starterP2pId, s.signMessage)
						if err != nil {
							errorCh <- err
							return
						}
						ticker.Stop()
						done <- true
					}
				}
			}
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
			logging.Infof("msg.name: {%s}", msg.Name)
			err := s.verify(msg)
			if err != nil {
				return err
			}
			switch msg.Name {
			case "register":
				if msg.Message != "" {
					err := s.operationSign.RegisterMessageHandler(rosenTss, msg, s.savedData.Ks, s.savedData.ShareID, s.signMessage)
					if err != nil {
						return err
					}
				}
			case "setup":
				if msg.Message != "" {
					err := s.operationSign.SetupMessageHandler(rosenTss, msg, s.savedData.Ks, s.signMessage)
					if err != nil {
						return err
					}
				}
			case "sign":
				if msg.Message != "" {
					//TODO: handle sign message
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
				err = s.operationSign.PartyUpdate(partyMsg)
				if err != nil {
					return err
				}
			case "startSign":
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
						result, err := s.operationSign.GossipMessageHandler(rosenTss, outCh, endCh, s.signMessage)
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
	return s.operationSign.SignMessage.Crypto + "Sign"
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
