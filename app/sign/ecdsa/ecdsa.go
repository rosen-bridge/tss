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

/*	Sign
	- creates private structure of ecdsa from save data,
	- gets blake_2b hash from message,
	- signs the hash
	args:
	message []byte
	returns:
	signature []byte, error
*/
func (h *handler) Sign(message []byte) ([]byte, error) {
	private := new(ecdsa.PrivateKey)
	private.PublicKey.Curve = tss.S256()
	private.D = h.savedData.Xi

	index, err := h.savedData.OriginalIndex()
	if err != nil {
		logging.Errorf("there was an error in finding index: %+v", err)
		return nil, err
	}

	private.PublicKey.X, private.PublicKey.Y = h.savedData.BigXj[index].X(), h.savedData.BigXj[index].Y()

	checksum := blake2b.Sum256(message)
	signature, err := private.Sign(rand.Reader, checksum[:], nil)
	if err != nil {
		logging.Errorf("there was an error in signing message: %+v", err)
		return nil, err
	}
	return signature, nil
}

/*	Verify
	- creates public structure of ecdsa from save data,
	- gets blake_2b hash from message,
	- validate the sign of message the hash
	args:
	msg []byte, sign_of_message []byte, index_of_peer_in_key_list  int
	returns:
	error
*/
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

/*	StartParty
	- creates tss parameters and party
	args:
	- local_tss_data *models.TssData,
	- list_of_peers_should_be_in_the_party tss.SortedPartyIDs,
	- threshold int,
	- data_should_be_signed *big.Int,
	- outCh chan tss.Message,
	- endCh chan common.SignatureData,
	returns:
	error
*/
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
		var localPartyId *tss.PartyID
		for _, peer := range peers {
			if peer.Id == localTssData.PartyID.Id {
				localPartyId = peer
			}
		}
		localTssData.Params = tss.NewParameters(tss.S256(), ctx, localPartyId, len(peers), threshold)
		localTssData.Party = ecdsaSigning.NewLocalParty(signData, localTssData.Params, h.savedData, outCh, endCh)
		if err := localTssData.Party.Start(); err != nil {
			return err
		}
		logging.Info("party started")
	}
	return nil
}

/*	LoadData
	- loads saved data from file for signing
	- creates tss party ID with p2pID
	args:
	app_interface_to_load_data _interface.RosenTss
	returns:
	party_Id *tss.PartyID, error
*/
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

/*	GetData
	- returns key_list and shared_ID of peer stored in the struct
	args:
	-
	returns:
	key_list []*big.Int, shared_id *big.Int
*/
func (h *handler) GetData() ([]*big.Int, *big.Int) {
	return h.savedData.Ks, h.savedData.ShareID
}
