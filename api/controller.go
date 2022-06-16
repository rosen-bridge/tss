package api

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"os/exec"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

// TssController Interface of an app controller
type TssController interface {
	Sign() echo.HandlerFunc
	Message() echo.HandlerFunc
	Export() echo.HandlerFunc
	Import() echo.HandlerFunc
	Keygen() echo.HandlerFunc
}

type tssController struct {
	rosenTss _interface.RosenTss
}

type response struct {
	Message string `json:"message"`
}

// NewTssController Constructor of an app controller
func NewTssController(rosenTss _interface.RosenTss) TssController {
	return &tssController{rosenTss: rosenTss}
}

// Keygen returns echo handler
func (tssController *tssController) Keygen() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.KeygenMessage{}

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		c.Logger().Info("keygen data: %+v ", data)

		err := tssController.rosenTss.StartNewKeygen(data)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		return c.JSON(http.StatusOK, response{
			Message: "ok",
		})
	}
}

// Sign returns echo handler, starting new sign process.
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

		return c.JSON(http.StatusOK, response{
			Message: "ok",
		})
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
		c.Logger().Infof("message data: %+v ", data)

		tssController.rosenTss.MessageHandler(data)

		return c.JSON(http.StatusOK, response{
			Message: "ok",
		})
	}
}

// Import returns echo handler witch used to using user key instead of generating a new key
func (tssController *tssController) Import() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.Private{}
		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		c.Logger().Info("import data: %+v ", data)
		err := tssController.rosenTss.SetPrivate(data)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, response{
			Message: "ok",
		})
	}
}

// Export returns echo handler witch used to download all files of app
func (tssController *tssController) Export() echo.HandlerFunc {
	return func(c echo.Context) error {
		peerHome := tssController.rosenTss.GetPeerHome()
		_, err := exec.Command("zip", "-r", "-D", "/tmp/rosenTss.zip", peerHome).Output()
		if err != nil {
			c.Logger().Errorf("%s", err)
		}
		c.Logger().Info("zipping file was successful.")
		return c.Attachment("/tmp/rosenTss.zip", "rosenTss.zip")
	}
}
