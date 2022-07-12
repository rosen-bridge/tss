package api

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"os"
	mockedApp "rosen-bridge/tss/mocks/app/interface"
	"rosen-bridge/tss/models"
	"strings"
	"testing"
)

/*	TestController_Sign
	TestCases :
	testing sign controller, there are 2 testcases 1 for ecdsa and 1 for eddsa.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is a models.SignMessage used as a function argument.
	Dependencies:
	it depends on `rosenTss.StartNewSign` function and will be handled as a mocked one.
*/
func TestController_Sign(t *testing.T) {

	tests := []struct {
		name        string
		signMessage models.SignMessage
	}{
		{
			name: "new eddsa signMessage, get status code 200",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
		},
		{
			name: "new ecdsa signMessage, get status code 200",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
		},
	}

	e := echo.New()
	app := mockedApp.NewRosenTss(t)
	app.On("StartNewSign", mock.AnythingOfType("models.SignMessage")).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewTssController(app)
			signHandler := controller.Sign()
			marshal, err := json.Marshal(tt.signMessage)
			if err != nil {
				return
			}
			req := httptest.NewRequest(http.MethodPost, "/sign", bytes.NewBuffer(marshal))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Assertions
			if assert.NoError(t, signHandler(c)) {
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			}
		})
	}

}

/*	TestController_Message
	TestCases:
	testing message controller, there is 1 testcase.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there is a models.Message used as a function argument.
	Dependencies:
	it depends on `rosenTss.MessageHandler` function and will be handled as a mocked one.
*/
func TestController_Message(t *testing.T) {

	tests := []struct {
		name    string
		message interface{}
		wantErr bool
	}{
		{
			name: "new true message, get status code 200",
			message: models.GossipMessage{
				Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
				SenderId:   "cahj2pgs4eqvn1eo1tp0",
				ReceiverId: "",
				Name:       "partyId",
			},
			wantErr: false,
		},
	}

	e := echo.New()
	app := mockedApp.NewRosenTss(t)
	app.On("MessageHandler", mock.AnythingOfType("models.Message")).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewTssController(app)
			messageHandler := controller.Message()
			marshal, err := json.Marshal(tt.message)
			if err != nil {
				t.Error(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/message", bytes.NewBuffer(marshal))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			// Assertions
			err = messageHandler(c)
			if err != nil && tt.wantErr {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			} else if err == nil && tt.wantErr {
				t.Error("false true")
			} else if err == nil && !tt.wantErr {
				assert.Equal(t, http.StatusOK, rec.Code)
			}
		})
	}

}

/*	TestController_Export
	TestCases:
	testing message controller, there is 1 testcase.
	the result should have status code 200.
	Dependencies:
	-
*/
func TestController_Export(t *testing.T) {
	// Setup, creating fake peer home with some files in it
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/export", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	app := mockedApp.NewRosenTss(t)
	peerHome := "/tmp/.rosenTss"
	app.On("GetPeerHome").Return(peerHome)

	if err := os.MkdirAll(peerHome, os.ModePerm); err != nil {
		t.Error(err)
	}
	_, err := os.Create(peerHome + "/test.txt")
	if err != nil {
		t.Error()
	}

	// calling controller
	controller := NewTssController(app)
	exportHandler := controller.Export()
	// Assertions
	if assert.NoError(t, exportHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}

/*	TestController_Import
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- rosenTss SetPrivate function
*/
func TestController_Import(t *testing.T) {
	// Setup
	type data struct {
		Private string `json:"private"`
		Crypto  string `json:"crypto"`
	}

	tests := []struct {
		name  string
		usage string
		data  interface{}
	}{
		{
			name: "success gossip",
			data: data{
				Private: "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d",
				Crypto:  "ecdsa",
			},
		},
		{
			name: "success meta",
			data: data{
				Private: "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d",
				Crypto:  "eddsa",
			},
		},
	}

	e := echo.New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			app := mockedApp.NewRosenTss(t)
			app.On("SetPrivate", mock.AnythingOfType("models.Private")).Return(nil)

			controller := NewTssController(app)
			importHandler := controller.Import()
			marshal, err := json.Marshal(tt.data)
			if err != nil {
				return
			}
			req := httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(string(marshal)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			// Assertions
			if assert.NoError(t, importHandler(c)) {
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			}
		})
	}

}

/*	TestController_Keygen
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are models.KeygenMessage used as test arguments.
	Dependencies:
	- rosenTss StartNewKeygen function
*/
func TestController_Keygen(t *testing.T) {
	// Setup

	tests := []struct {
		name          string
		keygenMessage models.KeygenMessage
	}{
		{
			name: "new eddsa keygenMessage",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "eddsa",
			},
		},
		{
			name: "success ecdsa keygenMessage",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "ecdsa",
			},
		},
	}

	e := echo.New()
	app := mockedApp.NewRosenTss(t)
	app.On("StartNewKeygen", mock.AnythingOfType("models.KeygenMessage")).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewTssController(app)
			keygenHandler := controller.Keygen()
			marshal, err := json.Marshal(tt.keygenMessage)
			if err != nil {
				return
			}
			req := httptest.NewRequest(http.MethodPost, "/keygen", bytes.NewBuffer(marshal))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			// Assertions
			if assert.NoError(t, keygenHandler(c)) {
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			}
		})
	}

}

/*	TestController_Regroup
	TestCases:
	testing message controller, there are 2 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	there are models.RegroupMessage used as test arguments.
	Dependencies:
	- rosenTss StartNewRegroup function
*/
func TestController_Regroup(t *testing.T) {
	// Setup

	tests := []struct {
		name           string
		regroupMessage models.RegroupMessage
	}{
		{
			name: "new eddsa RegroupMessage",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "eddsa",
			},
		},
		{
			name: "success ecdsa RegroupMessage",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "ecdsa",
			},
		},
	}

	e := echo.New()
	app := mockedApp.NewRosenTss(t)
	app.On("StartNewRegroup", mock.AnythingOfType("models.RegroupMessage")).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewTssController(app)
			regroupHandler := controller.Regroup()
			marshal, err := json.Marshal(tt.regroupMessage)
			if err != nil {
				return
			}
			req := httptest.NewRequest(http.MethodPost, "/regroup", bytes.NewBuffer(marshal))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			// Assertions
			if assert.NoError(t, regroupHandler(c)) {
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			}
		})
	}

}
