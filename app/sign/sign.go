package sign

import (
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

type OperationSign struct {
	_interface.OperationHandler
	LocalTssData models.TssData
	SignMessage  models.SignMessage
}
