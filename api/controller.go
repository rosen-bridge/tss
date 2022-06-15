package api

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

// TssController Interface of an app controller
type TssController interface {
	Sign() echo.HandlerFunc
	Message() echo.HandlerFunc
}

type tssController struct {
	rosenTss _interface.RosenTss
}

// NewTssController Constructor of an app controller
func NewTssController(rosenTss _interface.RosenTss) TssController {
	return &tssController{rosenTss: rosenTss}
}

//Sign returns echo handler, starting new sign process.
func (tssController *tssController) Sign() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.SignMessage{}

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		c.Logger().Info("sign data: %+v ", data)

		err := tssController.rosenTss.StartNewSign(data)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.String(http.StatusOK, "success")
	}
}

//Message returns echo handler, receiving message from p2p and passing to related channel
func (tssController *tssController) Message() echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Logger().Info("message called")

		var data models.Message

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		c.Logger().Info("sign data: %+v ", data)

		c.Logger().Infof("message data: %+v ", data)

		tssController.rosenTss.MessageHandler(data)

		return c.String(http.StatusOK, "success")
	}
}
