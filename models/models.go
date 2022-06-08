package models

import (
	"github.com/binance-chain/tss-lib/tss"
	"github.com/labstack/gommon/log"
)

var Logger = log.New("rosen-app")

type SignMessage struct {
	Crypto  string `json:"crypto"`
	Message string `json:"message"`
}

type Message struct {
	Message  GossipMessage
	Receiver string
	topic    string
}

type GossipMessage struct {
	Crypto    string `json:"crypto"`
	MessageId string `json:"messageId"` // keygen or sign or regroup
	Name      string `json:"name"`
	Message   string `json:"message"`
	SenderID  string `json:"senderId"`
}

type MetaData struct {
	PeersCount int `json:"peersCount"`
	Threshold  int `json:"threshold"`
}

type Private struct {
	ECDSAPrivate string `json:"ecdsaPrivate"`
	EDDSAPrivate string `json:"eddsaPrivate"`
}

type TssData struct {
	PartyID *tss.PartyID
	Params  *tss.Parameters
	Parties tss.SortedPartyIDs
	Party   tss.Party
}

type TssRegroupData struct {
	PartyID          *tss.PartyID
	Params           *tss.Parameters
	RegroupingParams *tss.ReSharingParameters
	NewParties       tss.SortedPartyIDs
	OldParties       tss.SortedPartyIDs
	Party            tss.Party
	PeerState        int
}

type PartyMessage struct {
	Message                 []byte
	GetFrom                 *tss.PartyID
	To                      []*tss.PartyID
	IsBroadcast             bool
	IsToOldCommittee        bool
	IsToOldAndNewCommittees bool
}
