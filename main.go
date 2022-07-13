package main

import (
	"flag"
	"fmt"

	logging "github.com/ipfs/go-log"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	_ "github.com/swaggo/echo-swagger/example/docs"
	"rosen-bridge/tss/api"
	"rosen-bridge/tss/app"
	"rosen-bridge/tss/models"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
)

var (
	cfgFile string
)

func init() {

}

func main() {

	// parsing cli flags
	projectPort := flag.String("port", "4000", "project port (e.g. 4000)")
	p2pPort := flag.String("p2pPort", "8080", "p2p port (e.g. 8080)")
	publishPath := flag.String(
		"publishPath", "/p2p/send", "publish path of p2p (e.g. /p2p/send)")
	subscriptionPath := flag.String(
		"subscriptionPath", "/p2p/channel/subscribe", "subscriptionPath for p2p (e.g. /p2p/channel/subscribe)")
	configFile := flag.String(
		"configFile", "./conf/conf.env", "config file")
	flag.Parse()

	err := initConfig(*configFile)
	if err != nil {
		panic(err)
	}

	logLevel := viper.GetString("LOG_LEVEL")

	err = configLog(logLevel)
	if err != nil {
		panic(err)
	}

	// creating new instance of echo framework
	e := echo.New()
	// initiating and reading configs

	// creating connection and storage and app instance
	conn := network.InitConnection(*publishPath, *subscriptionPath, *p2pPort)
	localStorage := storage.NewStorage()
	homeAddress := viper.GetString("HOME_ADDRESS")

	tss := app.NewRosenTss(conn, localStorage, homeAddress)

	// setting up peer home based on configs
	err = tss.SetPeerHome(homeAddress)
	if err != nil {
		e.Logger.Fatal(err)
	}

	// subscribe to p2p
	err = tss.GetConnection().Subscribe(*projectPort)
	if err != nil {
		e.Logger.Fatal(err)
	}

	// running echo framework
	tssController := api.NewTssController(tss)

	api.InitRouting(e, tssController)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", *projectPort)))
}

// initConfig reads in config file and ENV variables if set.
func initConfig(configFile string) error {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {

		// Search config in home directory with name "default" (without extension).
		viper.SetConfigFile(configFile)

	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		models.Logger.Info("Using config file:", viper.ConfigFileUsed())
	} else {
		return fmt.Errorf("error using config file: %s", err.Error())
	}
	return nil
}

func configLog(logLevel string) error {
	lvl, err := logging.LevelFromString(logLevel)
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)
	return nil
}
