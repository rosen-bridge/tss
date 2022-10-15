package ecdsa

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/binance-chain/tss-lib/common"
	ecdsaKeygen "github.com/binance-chain/tss-lib/ecdsa/keygen"
	ecdsaSigning "github.com/binance-chain/tss-lib/ecdsa/signing"
	"github.com/binance-chain/tss-lib/tss"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/app/sign"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/utils"
)

type operationECDSASign struct {
	sign.OperationSign
}

type handler struct {
	savedData ecdsaKeygen.LocalPartySaveData
}

var logging *zap.SugaredLogger
var ecdsaHandler handler

func NewSignECDSAOperation(signMessage models.SignMessage) _interface.Operation {
	logging = logger.NewSugar(signMessage.Crypto + "-sign")
	return &operationECDSASign{
		OperationSign: sign.OperationSign{
			SignMessage: signMessage,
			Signatures:  make(map[string]string),
			Logger:      logging,
			Handler:     ecdsaHandler,
		},
	}
}

// GetClassName returns the class name
func (s *operationECDSASign) GetClassName() string {
	return s.SignMessage.Crypto + "Sign"
}

func (s handler) Sign(message []byte) ([]byte, error) {
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

func (s handler) Verify(msg models.GossipMessage) error {
	public := new(ecdsa.PublicKey)
	public.Curve = tss.S256()
	public.X = s.savedData.BigXj[msg.Index].X()
	public.Y = s.savedData.BigXj[msg.Index].Y()

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

	result := ecdsa.VerifyASN1(public, checksum[:], msg.Signature)
	if !result {
		return fmt.Errorf("can not verify the message")
	}

	return nil
}

func (s handler) MessageHandler(
	rosenTss _interface.RosenTss, msg models.GossipMessage,
	signMessage string, localTssData models.TssData, operationSign *sign.OperationSign,
) error {

	errorCh := make(chan error, 1)

	msgBytes, _ := utils.Decoder(signMessage)
	signData := new(big.Int).SetBytes(msgBytes)

	logging.Info(
		"received startSign message: ",
		fmt.Sprintf("from: %s", msg.SenderId),
	)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan common.SignatureData, len(localTssData.PartyIds))
	for {
		if localTssData.Params == nil {
			time.Sleep(time.Second)
			continue
		} else {
			break
		}
	}

	if localTssData.Party == nil {
		localTssData.Party = ecdsaSigning.NewLocalParty(
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

func (s handler) LoadData(rosenTss _interface.RosenTss) (*tss.PartyID, error) {
	data, pID, err := rosenTss.GetStorage().LoadECDSAKeygen(rosenTss.GetPeerHome())
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

func (s handler) GetData() ([]*big.Int, *big.Int) {
	return s.savedData.Ks, s.savedData.ShareID
}
