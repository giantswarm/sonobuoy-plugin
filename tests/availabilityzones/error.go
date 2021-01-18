package availabilityzones

import (
	"github.com/giantswarm/microerror"
)

// This error should never be matched against and therefore there is no matcher
// implement. For further information see:
//
//     https://github.com/giantswarm/fmt/blob/master/go/errors.md#matching-errors
//
var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

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
