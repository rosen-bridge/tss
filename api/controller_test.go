package api

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	mockedApp "rosen-bridge/tss/mocks/app/interface"
	"rosen-bridge/tss/models"
	_ "rosen-bridge/tss/models"
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
			message: models.Message{
				Topic: "tss",
				Message: models.GossipMessage{
					Message:    "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
					MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
					SenderId:   "cahj2pgs4eqvn1eo1tp0",
					ReceiverId: "",
					Crypto:     "eddsa",
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