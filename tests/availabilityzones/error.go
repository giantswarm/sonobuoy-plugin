package availabilityzones

import (
	"github.com/giantswarm/microerror"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var missingEnvironmentVariable = &microerror.Error{
	Kind: "missingEnvironmentVariable",
}

var missingValueError = &microerror.Error{
	Kind: "missingValueError",
}
