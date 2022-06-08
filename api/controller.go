package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

// TODO: implement import and export handlers

// TssController Interface of an app controller
type TssController interface {
	Keygen() echo.HandlerFunc
	Sign() echo.HandlerFunc
	Regroup() echo.HandlerFunc
	Message() echo.HandlerFunc
	Import() echo.HandlerFunc
	Export() echo.HandlerFunc
}

type tssController struct {
	rosenTss _interface.RosenTss
}

// NewTssController Constructor of an app controller
func NewTssController(rosenTss _interface.RosenTss) TssController {
	return &tssController{rosenTss: rosenTss}
}

// Keygen returns echo handler
func (tssController *tssController) Keygen() echo.HandlerFunc {
	return func(c echo.Context) error {
		// TODO: implement this
		return c.String(http.StatusOK, "success")

	}

}

// Sign returns echo handler
func (tssController *tssController) Sign() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.SignMessage{}

		if err := c.Bind(&data); err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		log.Printf("sign data: %+v ", data)

		err := tssController.rosenTss.StartNewSign(data)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		return c.String(http.StatusOK, "success")
	}
}

// Regroup returns echo handler
func (tssController *tssController) Regroup() echo.HandlerFunc {
	return func(c echo.Context) error {
		// TODO: implement this
		return c.String(http.StatusOK, "success")
	}
}

//Message returns echo handler
func (tssController *tssController) Message() echo.HandlerFunc {
	return func(c echo.Context) error {
		// TODO: implement this
		return c.String(http.StatusOK, "success")
	}
}

// Import returns echo handler witch used to using user key instead of generating a new key
func (tssController *tssController) Import() echo.HandlerFunc {
	return func(c echo.Context) error {
		type data struct {
			Private string `json:"private"`
			Crypto  string `json:"crypto"`
		}
		response := data{}
		if err := c.Bind(&response); err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		log.Printf("import data: %+v ", response)

		// TODO: implement ImportPrivate
		//tssController.tssUseCase.ImportPrivate(response.Private, response.Crypto)
		return c.String(http.StatusOK, "success")
	}
}

// Export returns echo handler witch used to download all files of app
func (tssController *tssController) Export() echo.HandlerFunc {
	return func(c echo.Context) error {
		userHome, _ := os.UserHomeDir()
		projectHome := filepath.Join(userHome, ".app")

		out, err := exec.Command("zip", "-r", "-D", "/tmp/app.zip", projectHome).Output()
		if err != nil {
			models.Logger.Errorf("%s", err)
		}
		models.Logger.Info("zipping file was successful.")
		output := string(out[:])
		fmt.Println(output)
		return c.Attachment("/tmp/app.zip", "app.txt")
	}
}
