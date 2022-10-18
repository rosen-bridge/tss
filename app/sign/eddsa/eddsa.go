package eddsa

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/binance-chain/tss-lib/common"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	eddsaSigning "github.com/binance-chain/tss-lib/eddsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
)

type operationEDDSASign struct {
	sign.OperationSign
}

type handler struct {
	savedData eddsaKeygen.LocalPartySaveData
}

var logging *zap.SugaredLogger
var eddsaHandler handler

func NewSignEDDSAOperation(signMessage models.SignMessage) _interface.Operation {
	logging = logger.NewSugar("eddsa-sign")
	return &operationEDDSASign{
		OperationSign: sign.OperationSign{
			SignMessage: signMessage,
			Signatures:  make(map[string][]byte),
			Logger:      logging,
			Handler:     &eddsaHandler,
		},
	}
}

// GetClassName returns the class name
func (s *operationEDDSASign) GetClassName() string {
	return s.SignMessage.Crypto + "Sign"
}

func (s *handler) Sign(message []byte) ([]byte, error) {
	private, _, _ := edwards.PrivKeyFromScalar(s.savedData.Xi.Bytes())
	checksum := blake2b.Sum256(message)
	signature, err := private.Sign(checksum[:])
	if err != nil {
		return nil, err
	}
	return signature.Serialize(), nil
}

func (s *handler) Verify(msg models.GossipMessage) error {
	pk := edwards.NewPublicKey(s.savedData.BigXj[msg.Index].X(), s.savedData.BigXj[msg.Index].Y())
	payload := models.Payload{
		Message:   msg.Message,
		MessageId: msg.MessageId,
		SenderId:  msg.SenderId,
		Name:      msg.Name,
	}
	marshal, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	checksum := blake2b.Sum256(marshal)
	signature, err := edwards.ParseDERSignature(msg.Signature)
	if err != nil {
		return err
	}
	result := edwards.Verify(pk, checksum[:], signature.R, signature.S)
	if !result {
		return fmt.Errorf("can not verify the message")
	}
	return nil
}

func (s *handler) MessageHandler(
	rosenTss _interface.RosenTss, msg models.GossipMessage,
	signMessage string, localTssData models.TssData, operationSign *sign.OperationSign,
) error {

	errorCh := make(chan error, 1)

	meta := rosenTss.GetMetaData()
	msgBytes, _ := utils.Decoder(signMessage)
	signData := new(big.Int).SetBytes(msgBytes)

	logging.Info(
		"received startSign message: ",
		fmt.Sprintf("from: %s", msg.SenderId),
	)

	startSign := &models.StartSign{}
	err := json.Unmarshal([]byte(msg.Message), startSign)
	if err != nil {
		return err
	}

	_, ok := startSign.Signatures[localTssData.PartyID.Id]
	if !ok {
		return fmt.Errorf("this peer is not in the list of signatures")
	}

	if len(startSign.Signatures) < meta.Threshold {
		return fmt.Errorf("there is not eanough signature")
	}
	localTssData.PartyIds = startSign.Peers

	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))
	if localTssData.Party == nil {
		localTssData.Party = eddsaSigning.NewLocalParty(
			signData, localTssData.Params, s.savedData, outCh, endCh,
		)
		if err := localTssData.Party.Start(); err != nil {
			errorCh <- err
		}
		logging.Info("party started")

		result, err := operationSign.GossipMessageHandler(rosenTss, outCh, endCh)
		if err != nil {
			logging.Error(err)
			return err
		}
		if result {
			return fmt.Errorf("close channel")
		}
	}
	return nil
}

func (s *handler) LoadData(rosenTss _interface.RosenTss) (*tss.PartyID, error) {
	data, pID, err := rosenTss.GetStorage().LoadEDDSAKeygen(rosenTss.GetPeerHome())
	if err != nil {
		logging.Error(err)
		return nil, err
	}
	if pID == nil {
		logging.Error("pIDs is nil")
		return nil, err
	}
	s.savedData = data
	pID.Id = rosenTss.GetP2pId()
	return pID, nil
}

func (s *handler) GetData() ([]*big.Int, *big.Int) {
	return s.savedData.Ks, s.savedData.ShareID
}
