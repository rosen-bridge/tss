package keygen

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	eddsaKeygen "github.com/binance-chain/tss-lib/eddsa/keygen"
	"github.com/binance-chain/tss-lib/tss"
	"github.com/stretchr/testify/mock"
	"rosen-bridge/tss/app/interface"
	mockUtils "rosen-bridge/tss/mocks"
	mockedInterface "rosen-bridge/tss/mocks/app/interface"
	mockedNetwork "rosen-bridge/tss/mocks/network"
	mockedStorage "rosen-bridge/tss/mocks/storage"
	"rosen-bridge/tss/models"
	"strings"
	"testing"
	"time"
)

func TestEDDSA_Init(t *testing.T) {

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("CreateNewLocalEDDSATSSData error = %v", err)
	}

	app := mockedInterface.NewRosenTss(t)
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)
	app.On("GetPrivate", mock.AnythingOfType("string")).Return("4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d", nil)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromKeygen"),
			MessageId:  "keygen",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	tests := []struct {
		name         string
		app          _interface.RosenTss
		receiverId   string
		localTssData models.TssData
	}{
		{
			name:         "creating partyId message with localTssData",
			app:          app,
			receiverId:   "",
			localTssData: localTssData,
		},
		{
			name:         "creating partyId message without localTssData",
			app:          app,
			receiverId:   "cahj2pgs4eqvn1eo1tp0",
			localTssData: models.TssData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaKeygenOp := operationEDDSAKeygen{}
			err := eddsaKeygenOp.Init(tt.app, tt.receiverId)
			if err != nil {

				t.Errorf("Init failed: %v", err)
			}

		})
	}
}

func TestEDDSA_Loop(t *testing.T) {
	_, Id1, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}
	_, Id2, err := mockUtils.LoadEDDSAKeygenFixture(1)
	if err != nil {
		t.Errorf("LoadEDDSAKeygenFixture error = %v", err)
	}

	localTssData := models.TssData{
		PartyID: Id1,
	}
	newPartyId := Id2

	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), localTssData.PartyID))

	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.Edwards(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	party := eddsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	partyIDMessage := fmt.Sprintf("%s,%s,%d,%s", localTssData.PartyID.Id, localTssData.PartyID.Moniker, localTssData.PartyID.KeyInt(), "fromKeygen")

	partyMessage := models.PartyMessage{
		Message:     []byte(partyIDMessage),
		IsBroadcast: true,
		GetFrom:     newPartyId,
		To:          []*tss.PartyID{localTssData.PartyID},
	}

	partyMessageBytes, err := json.Marshal(partyMessage)
	if err != nil {
		t.Error("failed to marshal message", err)
	}

	tests := []struct {
		name    string
		message models.Message
	}{
		{
			name: "partyId",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    partyIDMessage,
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyId",
				},
			},
		},
		{
			name: "partyMsg",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    hex.EncodeToString(partyMessageBytes),
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyMsg",
				},
			},
		},
		{
			name: "keygen with party",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "generate key",
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "keygen",
				},
			},
		},
		{
			name: "keygen without party",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "generate key",
					MessageId:  "keygen",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "keygen",
				},
			},
		},
	}

	app := mockedInterface.NewRosenTss(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "partyId":
				app.On("GetMetaData").Return(
					models.MetaData{
						PeersCount: 3,
						Threshold:  2,
					})
			case "partyMsg", "keygen with party":
				localTssData.Party = party
			case "keygen without party":
				localTssData.Party = nil
				conn := mockedNetwork.NewConnection(t)
				conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
				app.On("GetConnection").Return(conn)
				app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(tt.message.Message)
			}

			eddsaKeygenOp := operationEDDSAKeygen{
				OperationKeygen: OperationKeygen{
					LocalTssData: localTssData,
				},
			}
			messageCh := make(chan models.Message, 100)

			messageCh <- tt.message
			go func() {
				time.Sleep(time.Second * 1)
				close(messageCh)
			}()
			errorList := []string{"invalid wire-format", "channel closed"}
			err := eddsaKeygenOp.Loop(app, messageCh)
			if err != nil && !mockUtils.Contains(err.Error(), errorList) {
				t.Error(err)
			}
		})
	}
}

func TestEDDSA_GetClassName(t *testing.T) {

	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "get eddsa class name",
			expected: "eddsaKeygen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaKeygenOp := operationEDDSAKeygen{}

			result := eddsaKeygenOp.GetClassName()
			if result != tt.expected {
				t.Errorf("GetClassName error = expected %s, got %s", tt.expected, result)
			}

		})
	}
}

func TestEDDSA_partyIdMessageHandler(t *testing.T) {
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	newPartyId2, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	var localTssData models.TssData
	localTssData.PartyID = newPartyId
	var localTssDataWith2PartyIds models.TssData
	localTssDataWith2PartyIds = localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId2))

	tests := []struct {
		name          string
		gossipMessage models.GossipMessage
		localTssData  models.TssData
	}{
		{
			name: "partyId message with t=1",
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromKeygen"),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			localTssData: localTssData,
		},
		{
			name: "partyId message with t=2",
			gossipMessage: models.GossipMessage{
				Message:    fmt.Sprintf("%s,%s,%d,%s", newPartyId.Id, newPartyId.Moniker, newPartyId.KeyInt(), "fromKeygen"),
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			localTssData: localTssDataWith2PartyIds,
		},
	}

	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app := mockedInterface.NewRosenTss(t)
	app.On("GetConnection").Return(conn)
	app.On("GetMetaData").Return(
		models.MetaData{
			Threshold:  2,
			PeersCount: 3,
		},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(tt.gossipMessage)
			eddsaKeygenOp := operationEDDSAKeygen{
				OperationKeygen: OperationKeygen{
					LocalTssData: tt.localTssData,
				},
			}
			// partyMessageHandler
			err := eddsaKeygenOp.partyIdMessageHandler(app, tt.gossipMessage)
			if err != nil {
				t.Error(err)
			}

		})
	}
}

func TestEDDSA_partyUpdate(t *testing.T) {

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Error(err)
	}

	newParty, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Error(err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newParty))

	ctx := tss.NewPeerContext(localTssData.PartyIds)
	localTssData.Params = tss.NewParameters(
		tss.S256(), ctx, localTssData.PartyID, len(localTssData.PartyIds), 1)
	outCh := make(chan tss.Message, len(localTssData.PartyIds))
	endCh := make(chan eddsaKeygen.LocalPartySaveData, len(localTssData.PartyIds))
	localTssData.Party = eddsaKeygen.NewLocalParty(localTssData.Params, outCh, endCh)

	for _, party := range localTssData.PartyIds {
		if party.Id == newParty.Id {
			newParty = party
			break
		}
	}

	tests := []struct {
		name    string
		message models.PartyMessage
		wantErr bool
	}{
		{
			name: "PartyUpdate from self to self",
			message: models.PartyMessage{
				To:                      []*tss.PartyID{localTssData.PartyID},
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: true,
		},
		{
			name: "PartyUpdate from new party to self",
			message: models.PartyMessage{
				To:                      []*tss.PartyID{localTssData.PartyID},
				GetFrom:                 newParty,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: false,
		},
		{
			name: "PartyUpdate from new party to all",
			message: models.PartyMessage{
				To:                      nil,
				GetFrom:                 newParty,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: false,
		},
		{
			name: "PartyUpdate from self to all",
			message: models.PartyMessage{
				To:                      nil,
				GetFrom:                 localTssData.PartyID,
				IsBroadcast:             true,
				Message:                 []byte("cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03"),
				IsToOldCommittee:        true,
				IsToOldAndNewCommittees: false,
			},
			wantErr: false,
		},
	}

	eddsaKeygenOp := operationEDDSAKeygen{
		OperationKeygen{
			LocalTssData: localTssData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := eddsaKeygenOp.partyUpdate(tt.message); (err != nil) != tt.wantErr {
				if !strings.Contains(err.Error(), "invalid wire-format data") {
					t.Errorf("PartyUpdate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

		})
	}
}

func TestEDDSA_setup(t *testing.T) {

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssDataWith2PartyIds := localTssData
	localTssDataWith2PartyIds.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	app := mockedInterface.NewRosenTss(t)
	app.On("GetMetaData").Return(
		models.MetaData{
			PeersCount: 3,
			Threshold:  2,
		})
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)
	tests := []struct {
		name         string
		app          _interface.RosenTss
		localTssData models.TssData
		wantErr      bool
	}{
		{
			name:         "creating keygen message partyId less than threshold",
			app:          app,
			localTssData: localTssData,
			wantErr:      true,
		},
		{
			name:         "creating keygen message with partyId equal to threshold",
			app:          app,
			localTssData: localTssDataWith2PartyIds,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaKeygenOp := operationEDDSAKeygen{
				OperationKeygen{
					LocalTssData: tt.localTssData,
				},
			}
			err := eddsaKeygenOp.setup(tt.app)
			if err != nil && !tt.wantErr {
				t.Errorf("Setup error = %v", err)
			}

		})
	}
}

func TestEDDSA_handleOutMessage(t *testing.T) {
	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}

	app := mockedInterface.NewRosenTss(t)
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(nil)
	app.On("GetConnection").Return(conn)
	tests := []struct {
		name         string
		app          _interface.RosenTss
		localTssData models.TssData
		tssMessage   tss.Message
	}{
		{
			name:         "creating party message",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaKeygenOp := operationEDDSAKeygen{
				OperationKeygen{
					LocalTssData: tt.localTssData,
				},
			}
			err := eddsaKeygenOp.handleOutMessage(tt.app, tt.tssMessage)
			if err != nil {
				t.Errorf("handleOutMessage error = %v", err)
			}

		})
	}
}

func TestEDDSA_handleEndMessage(t *testing.T) {

	fixture, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}

	tests := []struct {
		name       string
		keygenData eddsaKeygen.LocalPartySaveData
	}{
		{name: "creating eddsa keygen data", keygenData: fixture},
	}
	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	eddsaKeygenOp := operationEDDSAKeygen{}

	app := mockedInterface.NewRosenTss(t)
	app.On("GetStorage").Return(store)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eddsaKeygenOp.handleEndMessage(app, tt.keygenData)
			if err != nil {
				t.Errorf("handleEndMessage failed: %v", err)
			}
		})
	}

}

func TestEDDSA_gossipMessageHandler(t *testing.T) {
	fixture, _, err := mockUtils.LoadEDDSAKeygenFixture(0)
	if err != nil {
		t.Error(err)
	}

	message := mockUtils.TestUtilsMessage{
		Broadcast: true,
		Data:      "cfc72ea72b7e96bcf542ea2e359596031e13134d68a503cb13d3f31d8428ae03",
	}

	localTssData, err := mockUtils.CreateNewLocalEDDSATSSData()
	if err != nil {
		t.Errorf("createNewLocalEDDSAParty error = %v", err)
	}
	newPartyId, err := mockUtils.CreateNewEDDSAPartyId()
	if err != nil {
		t.Errorf("CreateNewEDDSAPartyId error = %v", err)
	}
	localTssData.PartyIds = tss.SortPartyIDs(
		append(localTssData.PartyIds.ToUnSorted(), newPartyId))

	store := mockedStorage.NewStorage(t)
	store.On("WriteData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("message received"))
	app := mockedInterface.NewRosenTss(t)
	app.On("GetPeerHome").Return("/tmp/.rosenTss")
	app.On("NewMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(models.GossipMessage{
			Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
			MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
			SenderId:   "cahj2pgs4eqvn1eo1tp0",
			ReceiverId: "",
			Name:       "partyId",
		})
	conn := mockedNetwork.NewConnection(t)
	conn.On("Publish", mock.AnythingOfType("models.GossipMessage")).Return(fmt.Errorf("message received"))
	app.On("GetConnection").Return(conn)
	app.On("GetStorage").Return(store)
	tests := []struct {
		name         string
		app          _interface.RosenTss
		keygenData   eddsaKeygen.LocalPartySaveData
		localTssData models.TssData
		tssMessage   tss.Message
	}{
		{
			name:         "save keygen",
			app:          app,
			keygenData:   fixture,
			localTssData: localTssData,
		},
		{
			name:         "party message",
			app:          app,
			localTssData: localTssData,
			tssMessage:   &message,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eddsaKeygenOp := operationEDDSAKeygen{
				OperationKeygen{LocalTssData: tt.localTssData},
			}
			outCh := make(chan tss.Message, len(eddsaKeygenOp.LocalTssData.PartyIds))
			endCh := make(chan eddsaKeygen.LocalPartySaveData, len(eddsaKeygenOp.LocalTssData.PartyIds))
			switch tt.name {
			case "save keygen":
				endCh <- tt.keygenData
			case "party message":
				outCh <- tt.tssMessage
			}
			err := eddsaKeygenOp.gossipMessageHandler(tt.app, outCh, endCh)
			if err != nil && err.Error() != "message received" {
				t.Errorf("gossipMessageHandler error = %v", err)
			}

		})
	}
}
