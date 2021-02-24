package sonobuoy_plugin

import (
	"github.com/giantswarm/microerror"
)

var appNotReadyError = &microerror.Error{
	Kind: "appNotReadyError",
}

var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

var notFoundError = &microerror.Error{
	Kind: "notFoundError",
}

var pvcUnboundError = &microerror.Error{
	Kind: "pvcUnboundError",
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
