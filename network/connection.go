package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"rosen-bridge/tss/models"
)

type Connection interface {
	Publish(message models.GossipMessage) error
	Subscribe(port string) error
	Unsubscribe() error
	CallBack(string, models.SignData) error
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type connect struct {
	publishUrl      string
	subscriptionUrl string
	subscribeId     string
	Client          HTTPClient
}

func InitConnection(publishPath string, subscriptionPath string, p2pPort string) Connection {
	publishUrl := fmt.Sprintf("http://localhost:%s%s", p2pPort, publishPath)
	subscriptionUrl := fmt.Sprintf("http://localhost:%s%s", p2pPort, subscriptionPath)
	return &connect{
		publishUrl:      publishUrl,
		subscriptionUrl: subscriptionUrl,
		Client:          &http.Client{},
	}

}

// Publish publishes a message to p2p
func (c *connect) Publish(msg models.GossipMessage) error {
	//models.Logger.Infof("message published: {%+v}", msg)

	values := models.Message{
		Message: msg,
		Topic:   "tss",
	}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.publishUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")

	resp, err := c.Client.Do(req)

	type response struct {
		Message string `json:"message"`
	}

	var res = response{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ok response: {%s}", res.Message)
	}
	if res.Message != "ok" {
		return fmt.Errorf("not ok response: {%s}", res.Message)
	}

	return nil
}

// Subscribe to p2p at first
func (c *connect) Subscribe(port string) error {
	models.Logger.Info("Subscribing to p2p")
	values := map[string]string{
		"channel": "tss",
		"url":     fmt.Sprintf("http://localhost:%s/message", port),
	}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.subscriptionUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")

	resp, err := c.Client.Do(req)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ok response: {%v}", resp.Body)
	}

	type response struct {
		Message string `json:"message"`
	}
	var res = response{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return err
	}
	if res.Message != "ok" {
		return fmt.Errorf("not ok response: {%s}", res.Message)
	}

	return nil
}

// CallBack sends sign data to this url

func (c *connect) CallBack(url string, data models.SignData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")

	resp, err := c.Client.Do(req)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ok response: {%v}", resp.Body)
	}
	return nil
}

func (c *connect) Unsubscribe() error {
	// TODO: implement this
	return nil
}
