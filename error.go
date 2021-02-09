package sonobuoy_plugin

import (
	"github.com/giantswarm/microerror"
)

var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

var notFoundError = &microerror.Error{
	Kind: "notFoundError",
}

// IsNotFound asserts notFoundError.
func IsNotFound(err error) bool {
	return microerror.Cause(err) == notFoundError
}

var unexpectedValueError = &microerror.Error{
	Kind: "unexpectedValueError",
}

// IsUnexpectedValueError asserts unexpectedValueError.
func IsUnexpectedValueError(err error) bool {
	return microerror.Cause(err) == unexpectedValueError
}
