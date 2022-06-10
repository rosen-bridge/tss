package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/binance-chain/tss-lib/common"
	"github.com/spf13/viper"
	"net/http"
	"rosen-bridge/tss/models"
)

type Connection interface {
	Publish(message models.GossipMessage) error
	Subscribe() error
	Unsubscribe() error
	CallBack(string, *common.SignatureData) error
}

type connect struct {
	publishUrl      string
	subscriptionUrl string
	subscribeId     string
}

func InitConnection() Connection {
	publishUrl := viper.GetString("PUBLISH_URL")
	subscriptionUrl := viper.GetString("SUBSCRIPTION_URL")

	return &connect{
		publishUrl:      publishUrl,
		subscriptionUrl: subscriptionUrl,
	}

}

// Publish publishes a message
func (c *connect) Publish(msg models.GossipMessage) error {
	models.Logger.Infof("message published: {%+v}", msg)

	values := models.Message{
		Message: msg,
		Topic:   "tss",
	}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return err
	}
	resp, err := http.Post(c.publishUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

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

func (c *connect) Subscribe() error {
	models.Logger.Info("Subscribing to p2p")
	port := viper.GetString("PORT")
	callBackURL := viper.GetString("CALLBACK_URL")
	values := map[string]string{
		"channel": "tss",
		"url":     fmt.Sprintf("%s:%s/message", callBackURL, port),
	}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.subscriptionUrl, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

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

func (c *connect) CallBack(url string, data *common.SignatureData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ok response: {%v}", resp.Body)
	}
	return nil
}
func (c *connect) Unsubscribe() error {
	// TODO: implement this
	return nil
}
