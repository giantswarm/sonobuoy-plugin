package sonobuoy_plugin

import (
	"github.com/giantswarm/microerror"
)

var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

var missingEnvironmentVariable = &microerror.Error{
	Kind: "missingEnvironmentVariable",
}
