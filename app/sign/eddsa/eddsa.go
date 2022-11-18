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
			Signatures:  make(map[int][]byte),
			Logger:      logging,
			Handler:     &eddsaHandler,
		},
	}
}

// GetClassName returns the class name
func (s *operationEDDSASign) GetClassName() string {
	return "eddsaSign"
}

/*	Sign
	- creates private structure of eddsa from save data,
	- gets blake_2b hash from message,
	- signs the hash
	args:
	message []byte
	returns:
	signature []byte, error
*/
func (h *handler) Sign(message []byte) ([]byte, error) {
	private, _, err := edwards.PrivKeyFromScalar(h.savedData.Xi.Bytes())
	if err != nil {
		logging.Errorf("there was an error in recovering private key: %+v", err)
		return nil, err
	}
	checksum := blake2b.Sum256(message)
	signature, err := private.Sign(checksum[:])
	if err != nil {
		logging.Errorf("there was an error in signing message: %+v", err)
		return nil, err
	}
	return signature.Serialize(), nil
}

/*	Verify
	- creates public structure of eddsa from save data,
	- gets blake_2b hash from message,
	- validate the sign of message the hash
	args:
	msg []byte, sign_of_message []byte, index_of_peer_in_key_list  int
	returns:
	error
*/
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
		localTssData.Params = tss.NewParameters(tss.Edwards(), ctx, localPartyId, len(peers), threshold)
		localTssData.Party = eddsaSigning.NewLocalParty(signData, localTssData.Params, h.savedData, outCh, endCh)
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
