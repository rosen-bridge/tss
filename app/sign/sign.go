package sign

import (
	"math/big"
	_interface "rosen-bridge/tss/app/interface"
	"rosen-bridge/tss/models"
)

const (
	signFileName = "sign_data.json"
)

type signOperation struct {
	_interface.OperationHandler
	LocalTssData models.TssData
	signData     *big.Int
}
