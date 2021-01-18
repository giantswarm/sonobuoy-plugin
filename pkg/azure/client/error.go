package client

import (
	"github.com/giantswarm/microerror"
	"strings"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(microerror.Cause(err).Error(), "ResourceGroupNotFound")
}
