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
	_ "rosen-bridge/tss/models"
	"strings"
	"testing"
)

func TestController_Sign(t *testing.T) {
	// Setup

	tests := []struct {
		name        string
		signMessage models.SignMessage
		wantErr     bool
	}{
		{
			name: "new eddsa signMessage",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
		},
		{
			name: "success ecdsa signMessage",
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
			} else if tt.wantErr {
				assert.Equal(t, http.StatusInternalServerError, rec.Code)
			}
		})
	}

}

func TestController_Message(t *testing.T) {
	// Setup
	tests := []struct {
		name    string
		message interface{}
		wantErr bool
	}{
		{
			name: "new true message",
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Name:       "partyId",
				},
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

func TestController_Export(t *testing.T) {
	// Setup
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

	controller := NewTssController(app)
	exportHandler := controller.Export()
	// Assertions
	if assert.NoError(t, exportHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		t.Log(rec.Body)
	}
}

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
