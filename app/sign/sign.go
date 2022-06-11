package sign

import (
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

const (
	signFileName = "sign_data.json"
)

type OperationSign struct {
	_interface.OperationHandler
	LocalTssData models.TssData
	SignMessage  models.SignMessage
}
