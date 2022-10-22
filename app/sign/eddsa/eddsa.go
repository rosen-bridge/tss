package eddsa

import (
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
	return "eddsaSign"
}

func (h *handler) Sign(message []byte) ([]byte, error) {
	private, _, _ := edwards.PrivKeyFromScalar(h.savedData.Xi.Bytes())
	checksum := blake2b.Sum256(message)
	signature, err := private.Sign(checksum[:])
	if err != nil {
		return nil, err
	}
	return signature.Serialize(), nil
}

func (h *handler) Verify(msg []byte, sign []byte, index int) error {
	pk := edwards.NewPublicKey(h.savedData.BigXj[index].X(), h.savedData.BigXj[index].Y())

	checksum := blake2b.Sum256(msg)
	signature, err := edwards.ParseDERSignature(sign)
	if err != nil {
		return err
	}
	result := edwards.Verify(pk, checksum[:], signature.R, signature.S)
	if !result {
		return fmt.Errorf("can not verify the message")
	}
	return nil
}

func (h *handler) StartParty(
	localTssData models.TssData,
	signData *big.Int,
	outCh chan tss.Message,
	endCh chan common.SignatureData,
) error {
	if localTssData.Party == nil {
		localTssData.Party = eddsaSigning.NewLocalParty(
			signData, localTssData.Params, h.savedData, outCh, endCh,
		)
		if err := localTssData.Party.Start(); err != nil {
			return err
		}
		logging.Info("party started")
	}
	return nil
}

func (h *handler) LoadData(rosenTss _interface.RosenTss) (*tss.PartyID, error) {
	data, pID, err := rosenTss.GetStorage().LoadEDDSAKeygen(rosenTss.GetPeerHome())
	if err != nil {
		logging.Error(err)
		return nil, err
	}
	if pID == nil {
		logging.Error("pIDs is nil")
		return nil, err
	}
	h.savedData = data
	pID.Id = rosenTss.GetP2pId()
	return pID, nil
}

func (h *handler) GetData() ([]*big.Int, *big.Int) {
	return h.savedData.Ks, h.savedData.ShareID
}
