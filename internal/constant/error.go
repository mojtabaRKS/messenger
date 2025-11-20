package constant

import "github.com/pkg/errors"

const InsufficientBalanceErrMsg = "insufficient balance"

var (
	InsufficientBalanceErr = errors.New(InsufficientBalanceErrMsg)
)
