package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	_interface "rosen-bridge/tss/app/interface"
	ecdsaKeygen "rosen-bridge/tss/app/keygen/ecdsa"
	eddsaKeygen "rosen-bridge/tss/app/keygen/eddsa"
	ecdsaSign "rosen-bridge/tss/app/sign/ecdsa"
	eddsaSign "rosen-bridge/tss/app/sign/eddsa"
	"rosen-bridge/tss/mocks"
	mockedApp "rosen-bridge/tss/mocks/app/interface"
	"rosen-bridge/tss/models"
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
		appConfig   func() _interface.RosenTss
		wantErr     bool
		statusCode  int
	}{
		{
			name: "new eddsa signMessage, get status code 200",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewSign", mock.AnythingOfType("models.SignMessage")).Return(nil)
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    false,
			statusCode: 200,
		},
		{
			name: "new ecdsa signMessage, get status code 200",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewSign", mock.AnythingOfType("models.SignMessage")).Return(nil)
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    false,
			statusCode: 200,
		},
		{
			name: "new ecdsa signMessage, get status code 409, duplicate messageId",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewSign",
					mock.AnythingOfType("models.SignMessage"),
				).Return(fmt.Errorf(models.DuplicatedMessageIdError))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "new ecdsa signMessage, get status code 400, no keygen data found",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewSign",
					mock.AnythingOfType("models.SignMessage"),
				).Return(fmt.Errorf(models.NoKeygenDataFoundError))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 400,
		},
		{
			name: "new ecdsa signMessage, get status code 500",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewSign", mock.AnythingOfType("models.SignMessage")).Return(fmt.Errorf("error"))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 500,
		},
		{
			name: "new ecdsa signMessage, get status code 409, there are other operations running",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "ecdsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				ECDSAOperation := ecdsaKeygen.NewKeygenECDSAOperation(models.KeygenMessage{})
				app.On("GetOperations").Return([]_interface.Operation{ECDSAOperation})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "new eddsa signMessage, get status code 409, there are other operations running",
			signMessage: models.SignMessage{
				Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
				Crypto:      "eddsa",
				CallBackUrl: "http://localhost:5050/callback/sign",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				EDDSAOperation := eddsaKeygen.NewKeygenEDDSAOperation(models.KeygenMessage{})
				app.On("GetOperations").Return([]_interface.Operation{EDDSAOperation})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
	}

	e := echo.New()
	logging, _ = mocks.InitLog("controller")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
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
				err = signHandler(c)
				e.HTTPErrorHandler(err, c)
				httpError, _ := err.(*echo.HTTPError)
				if err == nil {
					assert.Equal(t, http.StatusOK, rec.Code)
				} else {
					if tt.wantErr {
						t.Logf(err.Error())
						assert.Equal(t, tt.statusCode, rec.Code)
						if !strings.Contains(rec.Body.String(), httpError.Message.(string)) {
							t.Errorf("wrong error: %v", rec.Body.String())
						}
					} else {
						assert.Equal(t, http.StatusInternalServerError, rec.Code)
					}
				}

			},
		)
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
		name      string
		message   interface{}
		appConfig func() _interface.RosenTss
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
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("MessageHandler", mock.AnythingOfType("models.Message")).Return(nil)
				return app
			},
		},
	}

	e := echo.New()
	logging, _ = mocks.InitLog("controller")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
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
				e.HTTPErrorHandler(err, c)
				if err != nil {
					assert.Equal(t, http.StatusInternalServerError, rec.Code)
				}
			},
		)
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
	logging, _ = mocks.InitLog("controller")

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
	logging, _ = mocks.InitLog("controller")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {

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
			},
		)
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
		appConfig     func() _interface.RosenTss
		wantErr       bool
		statusCode    int
	}{
		{
			name: "new eddsa keygenMessage, statusCode 200",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "eddsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewKeygen", mock.AnythingOfType("models.KeygenMessage")).Return(nil)
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    false,
			statusCode: 200,
		},
		{
			name: "success eddsa keygenMessage, statusCode: 400",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewKeygen",
					mock.AnythingOfType("models.KeygenMessage"),
				).Return(fmt.Errorf(models.KeygenFileExistError))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 400,
		},
		{
			name: "success eddsa keygenMessage, statusCode: 500",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewKeygen", mock.AnythingOfType("models.KeygenMessage")).Return(fmt.Errorf("error"))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 500,
		},
		{
			name: "success eddsa keygenMessage, statusCode: 409, duplicate messageId",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewKeygen",
					mock.AnythingOfType("models.KeygenMessage"),
				).Return(fmt.Errorf(models.DuplicatedMessageIdError))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "success ecdsa keygenMessage, get status code 409, operation is running",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				ECDSAOperation := ecdsaSign.NewSignECDSAOperation(
					models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				)
				app.On("GetOperations").Return([]_interface.Operation{ECDSAOperation})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "success eddsa keygenMessage, get status code 409, operation is running",
			keygenMessage: models.KeygenMessage{
				Threshold:  2,
				PeersCount: 3,
				Crypto:     "eddsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				EDDSAOperation := eddsaSign.NewSignEDDSAOperation(
					models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				)
				app.On("GetOperations").Return([]_interface.Operation{EDDSAOperation})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
	}

	e := echo.New()
	logging, _ = mocks.InitLog("controller")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
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
				err = keygenHandler(c)
				e.HTTPErrorHandler(err, c)
				httpError, _ := err.(*echo.HTTPError)
				if err == nil {
					assert.Equal(t, http.StatusOK, rec.Code)
				} else {
					if tt.wantErr {
						t.Logf(err.Error())
						assert.Equal(t, tt.statusCode, rec.Code)
						if !strings.Contains(rec.Body.String(), httpError.Message.(string)) {
							t.Errorf("wrong error: %v", rec.Body.String())
						}
					} else {
						assert.Equal(t, http.StatusInternalServerError, rec.Code)
					}
				}
			},
		)
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
		appConfig      func() _interface.RosenTss
		wantErr        bool
		statusCode     int
	}{
		{
			name: "new eddsa RegroupMessage, statusCode: 200",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "eddsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewRegroup", mock.AnythingOfType("models.RegroupMessage")).Return(nil)
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    false,
			statusCode: 200,
		},
		{
			name: "success ecdsa RegroupMessage statusCode: 500",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("StartNewRegroup", mock.AnythingOfType("models.RegroupMessage")).Return(fmt.Errorf("error"))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 500,
		},
		{
			name: "success ecdsa RegroupMessage statusCode: 409, duplicate messageId",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewRegroup",
					mock.AnythingOfType("models.RegroupMessage"),
				).Return(fmt.Errorf(models.DuplicatedMessageIdError))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "eddsa, statusCode: 409, operation is running",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "eddsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				EDDSAOperation := eddsaSign.NewSignEDDSAOperation(
					models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "eddsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				)
				app.On("GetOperations").Return([]_interface.Operation{EDDSAOperation})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "ecdsa, statusCode: 409, operation is running",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				ECDSAOperation := ecdsaSign.NewSignECDSAOperation(
					models.SignMessage{
						Message:     "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
						Crypto:      "ecdsa",
						CallBackUrl: "http://localhost:5050/callback/sign",
					},
				)
				app.On("GetOperations").Return([]_interface.Operation{ECDSAOperation})
				return app
			},
			wantErr:    true,
			statusCode: 409,
		},
		{
			name: "ecdsa, statusCode: 400, No Keygen Data Found",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "ecdsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewRegroup",
					mock.AnythingOfType("models.RegroupMessage"),
				).Return(fmt.Errorf(models.NoKeygenDataFoundError))
				app.On("GetOperations").Return([]_interface.Operation{})

				return app
			},
			wantErr:    true,
			statusCode: 400,
		},
		{
			name: "eplsa, statusCode: 400, Wrong Crypto Protocol",
			regroupMessage: models.RegroupMessage{
				NewThreshold: 3,
				OldThreshold: 2,
				PeerState:    0,
				PeersCount:   4,
				Crypto:       "eplsa",
			},
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On(
					"StartNewRegroup",
					mock.AnythingOfType("models.RegroupMessage"),
				).Return(fmt.Errorf(models.WrongCryptoProtocolError))
				app.On("GetOperations").Return([]_interface.Operation{})
				return app
			},
			wantErr:    true,
			statusCode: 400,
		},
	}

	e := echo.New()
	logging, _ = mocks.InitLog("controller")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
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
				err = regroupHandler(c)
				e.HTTPErrorHandler(err, c)
				httpError, _ := err.(*echo.HTTPError)
				if err == nil {
					assert.Equal(t, http.StatusOK, rec.Code)
				} else {
					if tt.wantErr {
						t.Logf(err.Error())
						assert.Equal(t, tt.statusCode, rec.Code)
						if !strings.Contains(rec.Body.String(), httpError.Message.(string)) {
							t.Errorf("wrong error: %v", rec.Body.String())
						}
					} else {
						assert.Equal(t, http.StatusInternalServerError, rec.Code)
					}
				}
			},
		)
	}

}

/*	TestController_GetPk
	TestCases:
	testing message controller, there are 4 testcases.
	each test case runs as a subtests.
	target and expected outPut clarified in each testCase
	Dependencies:
	- rosenTss GetPublicKey function
*/
func TestController_GetPk(t *testing.T) {
	ecdsaPK, err := mocks.GetEcdsaPK()
	if err != nil {
		t.Error(err)
	}
	eddsaPK, err := mocks.GetEddsaPK()
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		name        string
		signMessage models.SignMessage
		appConfig   func() _interface.RosenTss
		wantErr     bool
		statusCode  int
	}{
		{
			name: "get eddsa pk",
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("GetPublicKey", mock.AnythingOfType("string")).Return(eddsaPK, nil)
				return app
			},
			wantErr:    false,
			statusCode: 200,
		},
		{
			name: "get ecdsa pk",
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("GetPublicKey", mock.AnythingOfType("string")).Return(ecdsaPK, nil)
				return app
			},
			wantErr:    false,
			statusCode: 200,
		},
		{
			name: "get wrong pk, error 500",
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("GetPublicKey", mock.AnythingOfType("string")).Return("", fmt.Errorf("wrong"))
				return app
			},
			wantErr:    true,
			statusCode: 500,
		},
		{
			name: "get wrong pk, error 400",
			appConfig: func() _interface.RosenTss {
				app := mockedApp.NewRosenTss(t)
				app.On("GetPublicKey", mock.AnythingOfType("string")).Return(
					"",
					fmt.Errorf(models.WrongCryptoProtocolError),
				)
				return app
			},
			wantErr:    true,
			statusCode: 400,
		},
	}

	e := echo.New()
	logging, _ = mocks.InitLog("controller")
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				app := tt.appConfig()
				controller := NewTssController(app)
				getPkHandler := controller.GetPk()
				marshal, err := json.Marshal(tt.signMessage)
				if err != nil {
					return
				}
				req := httptest.NewRequest(http.MethodGet, "/", bytes.NewBuffer(marshal))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)
				c.SetPath("/getPk/:crypto")
				c.SetParamNames("crypto")
				c.SetParamValues("ecdsa")

				// Assertions
				err = getPkHandler(c)
				e.HTTPErrorHandler(err, c)
				httpError, _ := err.(*echo.HTTPError)
				if err == nil {
					assert.Equal(t, http.StatusOK, rec.Code)
				} else {
					if tt.wantErr {
						t.Logf(err.Error())
						assert.Equal(t, tt.statusCode, rec.Code)
						if !strings.Contains(rec.Body.String(), httpError.Message.(string)) {
							t.Errorf("wrong error: %v", rec.Body.String())
						}
					} else {
						assert.Equal(t, http.StatusInternalServerError, rec.Code)
					}
				}

			},
		)
	}

}
