package regroup

import (
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

const (
	regroupFileName = "keygen_data.json"
)

type OperationRegroup struct {
	_interface.OperationHandler
	LocalTssData   models.TssRegroupData
	RegroupMessage models.RegroupMessage
}
