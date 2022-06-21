package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/labstack/echo/v4"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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

func errorHandler(code int, err string, c echo.Context) *echo.HTTPError {
	c.Logger().Error(err)
	return echo.NewHTTPError(code, err)
}

// Keygen returns echo handler
func (tssController *tssController) Keygen() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.KeygenMessage{}

		if err := c.Bind(&data); err != nil {
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
		}
		c.Logger().Info("keygen data: %+v ", data)

		err := tssController.rosenTss.StartNewKeygen(data)
		if err != nil {
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
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
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
		}
		c.Logger().Info("sign data: %+v ", data)

		err := tssController.rosenTss.StartNewSign(data)
		if err != nil {
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
		}

		return c.JSON(http.StatusOK, response{
			Message: "ok",
		})
	}
}

// Regroup returns echo handler
func (tssController *tssController) Regroup() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.RegroupMessage{}

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		c.Logger().Info("sign data: %+v ", data)

		err := tssController.rosenTss.StartNewRegroup(data)
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
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
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
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
		}
		c.Logger().Info("import data: %+v ", data)
		err := tssController.rosenTss.SetPrivate(data)
		if err != nil {
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
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
		// Create a buffer to write our archive to.
		c.Logger().Info("export called")
		buf := new(bytes.Buffer)

		// Create a new zip archive.
		zipWriter := zip.NewWriter(buf)

		var files []string
		err := filepath.Walk(peerHome, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
		}
		// Add some files to the archive.

		for _, file := range files {
			zipFile, err := zipWriter.Create(file)
			if err != nil {
				return errorHandler(http.StatusInternalServerError, err.Error(), c)
			}
			content, err := ioutil.ReadFile(file)
			if err != nil {
				return errorHandler(http.StatusInternalServerError, err.Error(), c)
			}
			_, err = zipFile.Write(content)
			if err != nil {
				return errorHandler(http.StatusInternalServerError, err.Error(), c)
			}
		}

		// Make sure to check the error on Close.
		err = zipWriter.Close()
		if err != nil {
			return errorHandler(http.StatusInternalServerError, err.Error(), c)
		}

		models.Logger.Info("zipping file was successful.")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("filename=%q", "rosenTss.zip"))
		return c.Stream(200, echo.HeaderContentDisposition, buf)
	}
}
