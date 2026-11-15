package delivery

import (
	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

var (
	ErrInput             = generationauthorization.ErrInput
	ErrBinding           = generationauthorization.ErrBinding
	ErrAuthorization     = generationauthorization.ErrAuthorization
	ErrNativeUnavailable = requestinput.ErrNativeUnavailable
)
