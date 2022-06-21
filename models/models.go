package models

import (
	"github.com/binance-chain/tss-lib/tss"
	"github.com/labstack/gommon/log"
)

var Logger = log.New("rosen-app")

type SignMessage struct {
	Crypto      string `json:"crypto"`
	Message     string `json:"message"`
	CallBackUrl string `json:"callBackUrl"`
}

type SignData struct {
	Signature string `json:"signature"`
	R         string `json:"r"`
	S         string `json:"s"`
	M         string `json:"m"`
}

type Message struct {
	Message GossipMessage `json:"message"`
	Sender  string        `json:"sender"`
	Topic   string        `json:"channel"`
}

type GossipMessage struct {
	MessageId  string `json:"messageId"` // keygen or sign or regroup
	Name       string `json:"name"`
	Message    string `json:"message"`
	SenderId   string `json:"senderId"`
	ReceiverId string `json:"receiverId"`
}

type MetaData struct {
	PeersCount int `json:"peersCount"`
	Threshold  int `json:"threshold"`
}

type Private struct {
	Private string `json:"private"`
	Crypto  string `json:"crypto"`
}

type TssData struct {
	PartyID  *tss.PartyID
	Params   *tss.Parameters
	PartyIds tss.SortedPartyIDs
	Party    tss.Party
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

type KeygenMessage struct {
	PeersCount int    `json:"peersCount"`
	Threshold  int    `json:"threshold"`
	Crypto     string `json:"crypto"`
}
