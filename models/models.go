package models

import (
	"github.com/binance-chain/tss-lib/tss"
)

const (
	DuplicatedMessageIdError = "duplicated messageId"
	KeygenFileExistError     = "keygen file exists"
	OperationIsRunningError  = "operation is running"
	NoKeygenDataFoundError   = "no keygen data found"
	WrongCryptoProtocolError = "wrong crypto protocol"
	NotPartOfSigningProcess  = "this party is not part of signing process"
)

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
	Message string `json:"message"`
	Sender  string `json:"sender"`
	Topic   string `json:"channel"`
}

type GossipMessage struct {
	MessageId  string `json:"messageId"`
	Name       string `json:"name"`
	Message    string `json:"message"`
	SenderId   string `json:"senderId"`
	ReceiverId string `json:"receiverId"`
	Signature  []byte `json:"signature"`
	Index      int    `json:"index"`
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
	RegroupingParams *tss.ReSharingParameters
	NewPartyIds      tss.SortedPartyIDs
	OldPartyIds      tss.SortedPartyIDs
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

type RegroupMessage struct {
	PeerState    int    `json:"peerState"` // 0 for old and 1 for new
	Crypto       string `json:"crypto"`
	NewThreshold int    `json:"newThreshold"`
	OldThreshold int    `json:"oldThreshold"`
	PeersCount   int    `json:"peersCount"`
	CallBackUrl  string `json:"callBackUrl"`
}

type KeygenMessage struct {
	PeersCount  int    `json:"peersCount"`
	Threshold   int    `json:"threshold"`
	Crypto      string `json:"crypto"`
	CallBackUrl string `json:"callBackUrl"`
}

type Config struct {
	HomeAddress                string  `mapstructure:"HOME_ADDRESS"`
	LogLevel                   string  `mapstructure:"LOG_LEVEL"`
	LogMaxSize                 int     `mapstructure:"LOG_MAX_SIZE"`
	LogMaxBackups              int     `mapstructure:"LOG_MAX_BACKUPS"`
	LogMaxAge                  int     `mapstructure:"LOG_MAX_AGE"`
	OperationTimeout           int     `mapstructure:"OPERATION_TIMEOUT"`
	MessageTimeout             int     `mapstructure:"MESSAGE_TIMEOUT"`
	LeastProcessRemainingTime  int64   `mapstructure:"LEAST_PROCESS_REMAINING_TIME"`
	SetupBroadcastInterval     int64   `mapstructure:"SETUP_BROADCAST_INTERVAL"`
	SignStartTimeTracker       float64 `mapstructure:"SIGN_START_TIME_TRACKER"`
	TurnDuration               int64   `mapstructure:"TRUN_DURATION"`
	WaitInPartyMessageHandling int64   `mapstructure:"WAIT_IN_PARTY_MESSAGE_HANDLING"`
}

type Register struct {
	Id        string `json:"id"`
	Moniker   string `json:"moniker"`
	Key       string `json:"key"`
	Timestamp int64  `json:"timestamp"`
	NoAnswer  bool   `json:"noAnswer"`
}

type Payload struct {
	MessageId string `json:"messageId"`
	Name      string `json:"name"`
	Message   string `json:"message"`
	SenderId  string `json:"senderId"`
}

type SetupSign struct {
	Hash      string        `json:"hash"`
	Peers     []tss.PartyID `json:"peers"`
	Timestamp int64         `json:"timestamp"`
	StarterId string        `json:"starterId"`
}

type StartSign struct {
	Hash       string         `json:"hash"`
	Signatures map[int][]byte `json:"signatures"`
	StarterId  string         `json:"starterId"`
	Peers      []tss.PartyID  `json:"peers"`
	Timestamp  int64          `json:"timestamp"`
}
