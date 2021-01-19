package ctrlclient

import (
	"github.com/giantswarm/microerror"
)

var missingEnvironmentVariable = &microerror.Error{
	Kind: "missingEnvironmentVariable",
}
