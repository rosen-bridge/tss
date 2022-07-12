package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	mockedClient "rosen-bridge/tss/mocks/client"
	"rosen-bridge/tss/models"
	"testing"
)

/*	TestConnection_Publish
	TestCases:
	test for publish route,
	there must be no error and the response code and response type must bo correct
	Dependencies:
	- http client
*/
func TestConnection_Publish(t *testing.T) {
	// test for publish route, there must be no error and the response code and response type must bo correct
	message := models.GossipMessage{
		Message:    "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		MessageId:  "ccd5480560cf2dec4098917b066264f28cd5b648358117cfdc438a7b165b3bb1",
		SenderId:   "1",
		ReceiverId: "",
		Name:       "partyId",
	}

	cnn := connect{
		publishUrl:      "http://localhost:8080/p2p/send",
		subscriptionUrl: "http://localhost:8080/p2p/channel/subscribe",
	}

	cnn.Client = &mockedClient.MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/p2p/send" {
				t.Errorf("Expected to request '/p2p/send', got: %s", req.URL.Path)
			}
			if req.Header.Get("content-type") != "application/json" {
				t.Errorf("Expected content-type: application/json header, got: %s", req.Header.Get("content-type"))
			}

			response := map[string]string{
				"message": "ok",
			}
			jsonMessage, err := json.Marshal(response)
			if err != nil {
				t.Error(err)
			}
			responseBody := ioutil.NopCloser(bytes.NewReader(jsonMessage))
			return &http.Response{
				StatusCode: 200,
				Body:       responseBody,
			}, nil
		},
	}

	err := cnn.Publish(message)
	if err != nil {
		t.Error(err)
	}
}

/*	TestConnection_Subscribe
	TestCases:
	test for Subscribe route,
	there must be no error and the response code and response type must bo correct
	Dependencies:
	- http client
*/
func TestConnection_Subscribe(t *testing.T) {

	cnn := connect{
		publishUrl:      "http://localhost:8080/p2p/send",
		subscriptionUrl: "http://localhost:8080/p2p/channel/subscribe",
	}
	projectPort := "4000"
	cnn.Client = &mockedClient.MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/p2p/channel/subscribe" {
				t.Errorf("Expected to request '/p2p/channel/subscribe', got: %s", req.URL.Path)
			}
			if req.Header.Get("content-type") != "application/json" {
				t.Errorf("Expected content-type: application/json header, got: %s", req.Header.Get("content-type"))
			}

			response := map[string]string{
				"message": "ok",
			}
			jsonMessage, err := json.Marshal(response)
			if err != nil {
				t.Error(err)
			}
			responseBody := ioutil.NopCloser(bytes.NewReader(jsonMessage))
			return &http.Response{
				StatusCode: 200,
				Body:       responseBody,
			}, nil
		},
	}

	err := cnn.Subscribe(projectPort)
	if err != nil {
		t.Error(err)
	}
}

/*	TestConnection_CallBack
	TestCases:
	test for CallBack route,
	there must be no error and the response code and response type must bo correct
	Dependencies:
	- http client
*/
func TestConnection_CallBack(t *testing.T) {
	cnn := connect{
		publishUrl:      "http://localhost:8080/p2p/send",
		subscriptionUrl: "http://localhost:8080/p2p/channel/subscribe",
	}
	callBackURLHost := "http://localhost:5050"
	callBackURLPath := "/"

	r := "b24c712530dd03739ac87a491e45bd80ea8e3cef19c835bc6ed3262a9794974d"
	s := "02fe41c73871ca7ded0ff3e8adc76a64ea93643e75569bd9db8f772166adfc35"
	m := "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70"
	signature := "4d9794972a26d36ebc35c819ef3c8eea80bd451e497ac89a7303dd3025714cb235fcad6621778fdbd99b56753e6493ea646ac7ade8f30fed7dca7138c741fe02"
	saveSign := models.SignData{
		R:         r,
		S:         s,
		M:         m,
		Signature: signature,
	}

	cnn.Client = &mockedClient.MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != callBackURLPath {
				t.Errorf("Expected to request '%s', got: %s", callBackURLPath, req.URL.Path)
			}
			if req.Header.Get("content-type") != "application/json" {
				t.Errorf("Expected content-type: application/json header, got: %s", req.Header.Get("content-type"))
			}

			response := map[string]string{
				"message": "ok",
			}
			jsonMessage, err := json.Marshal(response)
			if err != nil {
				t.Error(err)
			}
			responseBody := ioutil.NopCloser(bytes.NewReader(jsonMessage))
			return &http.Response{
				StatusCode: 200,
				Body:       responseBody,
			}, nil
		},
	}

	url := fmt.Sprintf("%s%s", callBackURLHost, callBackURLPath)
	err := cnn.CallBack(url, saveSign)
	if err != nil {
		t.Error(err)
	}
}
