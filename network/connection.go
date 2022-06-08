package network

import (
	"github.com/spf13/viper"
	"rosen-bridge/tss/models"
)

type Connection interface {
	Publish(message []byte) error
	Subscribe() error
	Unsubscribe() error
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
func (c *connect) Publish(msg []byte) error {
	// TODO: implement this
	models.Logger.Infof("message published: {%v}", string(msg))
	return nil
}

func (c *connect) Subscribe() error {
	// TODO: implement this
	c.subscribeId = ""
	return nil
}

func (c *connect) Unsubscribe() error {
	// TODO: implement this
	return nil
}
