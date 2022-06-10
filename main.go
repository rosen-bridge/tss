package main

import (
	"fmt"
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

func main() {
	e := echo.New()

	err := initConfig()
	if err != nil {
		e.Logger.Fatal(err)
	}

	homeAddress := viper.GetString("HOME_ADDRESS")
	port := viper.GetString("PORT")

	conn := network.InitConnection()
	localStorage := storage.NewStorage()

	tss := app.NewRosenTss(conn, localStorage, homeAddress)

	err = tss.SetPeerHome(homeAddress)
	if err != nil {
		e.Logger.Fatal(err)
	}

	// subscribe to p2p
	err = tss.GetConnection().Subscribe()
	if err != nil {
		e.Logger.Fatal(err)
	}

	tssController := api.NewTssController(tss)

	api.InitRouting(e, tssController)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {

		// Search config in home directory with name "default" (without extension).
		viper.AddConfigPath("./conf")
		viper.SetConfigName("conf")
		viper.SetConfigType("env")

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
