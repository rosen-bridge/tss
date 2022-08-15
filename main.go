package main

import (
	"flag"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	_ "github.com/swaggo/echo-swagger/example/docs"
	"rosen-bridge/tss/api"
	"rosen-bridge/tss/app"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/network"
	"rosen-bridge/tss/storage"
	"rosen-bridge/tss/utils"
)

var (
	cfgFile string
)

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

	homeAddress := viper.GetString("HOME_ADDRESS")
	absAddress, err := utils.GetAbsoluteAddress(homeAddress)
	if err != nil {
		panic(err)
	}
	logFile := fmt.Sprintf("%s/%s", absAddress, "tss.log")
	logLevel := viper.GetString("LOG_LEVEL")
	MaxSize := viper.GetInt("LOG_MAX_SIZE")
	MaxBackups := viper.GetInt("LOG_MAX_BACKUPS")
	MaxAge := viper.GetInt("LOG_MAX_AGE")

	err = logger.Init(logFile, logLevel, MaxSize, MaxBackups, MaxAge, false)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := logger.Sync()
		if err != nil {

		}
	}()

	logging := logger.NewSugar("main")

	// creating new instance of echo framework
	e := echo.New()
	// initiating and reading configs

	// creating connection and storage and app instance
	conn := network.InitConnection(*publishPath, *subscriptionPath, *p2pPort)
	localStorage := storage.NewStorage()

	tss := app.NewRosenTss(conn, localStorage, homeAddress)

	// setting up peer home based on configs
	err = tss.SetPeerHome(homeAddress)
	if err != nil {
		logging.Fatal(err)
	}

	// subscribe to p2p
	err = tss.GetConnection().Subscribe(*projectPort)
	if err != nil {
		logging.Error(err)
	}

	// running echo framework
	tssController := api.NewTssController(tss)

	api.InitRouting(e, tssController)
	logging.Fatal(e.Start(fmt.Sprintf(":%s", *projectPort)))
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
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error using config file: %s", err.Error())
	}
	return nil
}
