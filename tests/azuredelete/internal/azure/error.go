package azure

import (
	"github.com/giantswarm/microerror"
	"strings"
)

var missingEnvVarError = &microerror.Error{
	Kind: "missingEnvVarError",
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(microerror.Cause(err).Error(), "ResourceGroupNotFound")
}
