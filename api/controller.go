package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/logger"
	"rosen-bridge/tss/models"
)

// TssController Interface of an app controller
type TssController interface {
	Sign() echo.HandlerFunc
	Message() echo.HandlerFunc
	Export() echo.HandlerFunc
	Import() echo.HandlerFunc
	Keygen() echo.HandlerFunc
	Regroup() echo.HandlerFunc
	GetPk() echo.HandlerFunc
}

type tssController struct {
	rosenTss _interface.RosenTss
}

type response struct {
	Message string `json:"message"`
}

var logging *zap.SugaredLogger

// NewTssController Constructor of an app controller
func NewTssController(rosenTss _interface.RosenTss) TssController {
	logging = logger.NewSugar("controller")
	return &tssController{
		rosenTss: rosenTss,
	}
}

// checkOperation check if there is any common between forbidden list of requested operation and running operations
func (tssController *tssController) checkOperation(forbiddenOperations []string) error {
	operations := tssController.rosenTss.GetOperations()
	for _, operation := range operations {
		for _, forbidden := range forbiddenOperations {
			if operation.GetClassName() == forbidden {
				return fmt.Errorf("%s "+models.OperationIsRunningError, forbidden)
			}
		}
	}
	return nil
}

// Keygen returns echo handler, starting new keygen process
func (tssController *tssController) Keygen() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.KeygenMessage{}

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		logging.Infof("keygen data: %+v ", data)

		forbiddenOperations := []string{data.Crypto + "Sign"}
		err := tssController.checkOperation(forbiddenOperations)
		if err != nil {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		err = tssController.rosenTss.StartNewKeygen(data)
		if err != nil {
			switch err.Error() {
			case models.DuplicatedMessageIdError:
				return echo.NewHTTPError(http.StatusConflict, err.Error())
			case models.KeygenFileExistError, models.WrongCryptoProtocolError:
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			default:
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
		}
		return c.JSON(
			http.StatusOK, response{
				Message: "ok",
			},
		)
	}
}

// Sign returns echo handler, starting new sign process.
func (tssController *tssController) Sign() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.SignMessage{}

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		logging.Infof("sign data: %+v ", data)

		forbiddenOperations := []string{data.Crypto + "Keygen", data.Crypto + "Regroup"}
		err := tssController.checkOperation(forbiddenOperations)
		if err != nil {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		err = tssController.rosenTss.StartNewSign(data)
		if err != nil {
			switch err.Error() {
			case models.DuplicatedMessageIdError:
				return echo.NewHTTPError(http.StatusConflict, err.Error())
			case models.NoKeygenDataFoundError, models.WrongCryptoProtocolError:
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			default:
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
		}

		return c.JSON(
			http.StatusOK, response{
				Message: "ok",
			},
		)
	}
}

// Regroup returns echo handler, starting new regroup process
func (tssController *tssController) Regroup() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.RegroupMessage{}

		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		logging.Infof("regroup data: %+v ", data)

		forbiddenOperations := []string{data.Crypto + "Sign"}
		err := tssController.checkOperation(forbiddenOperations)
		if err != nil {
			return echo.NewHTTPError(http.StatusConflict, err.Error())
		}
		err = tssController.rosenTss.StartNewRegroup(data)
		if err != nil {
			switch err.Error() {
			case models.DuplicatedMessageIdError:
				return echo.NewHTTPError(http.StatusConflict, err.Error())
			case models.NoKeygenDataFoundError, models.WrongCryptoProtocolError:
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			default:
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
		}

		return c.JSON(
			http.StatusOK, response{
				Message: "ok",
			},
		)
	}
}

//Message returns echo handler, receiving message from p2p and passing to related channel
func (tssController *tssController) Message() echo.HandlerFunc {
	return func(c echo.Context) error {
		var data models.Message
		logging.Infof("message route called")
		if err := c.Bind(&data); err != nil {
			logging.Errorf("can not bind data, err: %+v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		err := tssController.rosenTss.MessageHandler(data)
		if err != nil {
			logging.Error(err)
		}
		return c.JSON(
			http.StatusOK, response{
				Message: "ok",
			},
		)
	}
}

// Import returns echo handler witch used to using user key instead of generating a new key
func (tssController *tssController) Import() echo.HandlerFunc {
	return func(c echo.Context) error {
		data := models.Private{}
		if err := c.Bind(&data); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		logging.Info("importing data")
		err := tssController.rosenTss.SetPrivate(data)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(
			http.StatusOK, response{
				Message: "ok",
			},
		)
	}
}

// Export returns echo handler witch used to download all files of app
func (tssController *tssController) Export() echo.HandlerFunc {
	return func(c echo.Context) error {
		peerHome := tssController.rosenTss.GetPeerHome()
		// Create a buffer to write our archive to.
		logging.Info("export called")
		buf := new(bytes.Buffer)

		// Create a new zip archive.
		zipWriter := zip.NewWriter(buf)

		var files []string
		err := filepath.Walk(
			peerHome, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}
				files = append(files, path)
				return nil
			},
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		// Add some files to the archive.

		for _, file := range files {
			zipFile, err := zipWriter.Create(file)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			content, err := ioutil.ReadFile(file)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			_, err = zipFile.Write(content)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
		}

		// Make sure to check the error on Close.
		err = zipWriter.Close()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		logging.Info("zipping file was successful.")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("filename=%q", "rosenTss.zip"))
		return c.Stream(200, echo.HeaderContentDisposition, buf)
	}
}

// GetPk returns echo handler witch used to get generated public key in keygen process base on crypto protocol
func (tssController *tssController) GetPk() echo.HandlerFunc {
	return func(c echo.Context) error {
		crypto := c.QueryParam("crypto")

		logging.Infof("get %s public key for", crypto)

		pk, err := tssController.rosenTss.GetPublicKey(crypto)
		if err != nil {
			switch err.Error() {
			case models.NoKeygenDataFoundError, models.WrongCryptoProtocolError:
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			default:
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
		}

		res := map[string]string{"message": "ok", "pubKey": pk}
		return c.JSON(http.StatusOK, res)
	}
}
