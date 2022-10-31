package ecdsa

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"

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
	logging = logger.NewSugar("ecdsa-sign")
	return &operationECDSASign{
		OperationSign: sign.OperationSign{
			SignMessage: signMessage,
			Signatures:  make(map[int][]byte),
			Logger:      logging,
			Handler:     &ecdsaHandler,
		},
	}
}

// GetClassName returns the class name
func (s *operationECDSASign) GetClassName() string {
	return "ecdsaSign"
}

func (h *handler) Sign(message []byte) ([]byte, error) {
	private := new(ecdsa.PrivateKey)
	private.PublicKey.Curve = tss.S256()
	private.D = h.savedData.Xi

	index, err := h.savedData.OriginalIndex()
	if err != nil {
		return nil, err
	}

	private.PublicKey.X, private.PublicKey.Y = h.savedData.BigXj[index].X(), h.savedData.BigXj[index].Y()

	checksum := blake2b.Sum256(message)
	signature, err := private.Sign(rand.Reader, checksum[:], nil)
	if err != nil {
		panic(err)
	}
	return signature, nil
}

func (h *handler) Verify(msg []byte, sign []byte, index int) error {
	public := new(ecdsa.PublicKey)
	public.Curve = tss.S256()
	public.X = h.savedData.BigXj[index].X()
	public.Y = h.savedData.BigXj[index].Y()

	checksum := blake2b.Sum256(msg)

	result := ecdsa.VerifyASN1(public, checksum[:], sign)
	if !result {
		return fmt.Errorf("can not verify the message")
	}

	return nil
}

func (h *handler) StartParty(
	localTssData *models.TssData,
	peers tss.SortedPartyIDs,
	threshold int,
	signData *big.Int,
	outCh chan tss.Message,
	endCh chan common.SignatureData,
) error {
	if localTssData.Party == nil {
		ctx := tss.NewPeerContext(peers)
		logging.Info("creating party parameters")
		localTssData.Params = tss.NewParameters(tss.S256(), ctx, localTssData.PartyID, len(peers), threshold)

		localTssData.Party = ecdsaSigning.NewLocalParty(signData, localTssData.Params, h.savedData, outCh, endCh)
		if err := localTssData.Party.Start(); err != nil {
			return err
		}
		logging.Info("party started")
	}
	return nil
}

func (h *handler) LoadData(rosenTss _interface.RosenTss) (*tss.PartyID, error) {
	data, pID, err := rosenTss.GetStorage().LoadECDSAKeygen(rosenTss.GetPeerHome())
	if err != nil {
		logging.Error(err)
		return nil, err
	}
	if pID == nil {
		logging.Error("pIDs is nil")
		return nil, err
	}
	h.savedData = data
	pID.Moniker = fmt.Sprintf("tssPeer/%s", rosenTss.GetP2pId())
	pID.Id = rosenTss.GetP2pId()
	return pID, nil
}

func (h *handler) GetData() ([]*big.Int, *big.Int) {
	return h.savedData.Ks, h.savedData.ShareID
}
