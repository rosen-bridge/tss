package keygen

import (
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

const (
	KeygenFileName = "keygen_data.json"
)

type OperationKeygen struct {
	_interface.OperationHandler
	LocalTssData  models.TssData
	KeygenMessage models.KeygenMessage
}
