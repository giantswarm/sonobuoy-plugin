package sonobuoy_plugin

import (
	"github.com/giantswarm/microerror"
)

var appNotReadyError = &microerror.Error{
	Kind: "appNotReadyError",
}

var dsNotReadyError = &microerror.Error{
	Kind: "dsNotReadyError",
}

var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

var notFoundError = &microerror.Error{
	Kind: "notFoundError",
}

var podNotReadyError = &microerror.Error{
	Kind: "podNotReadyError",
}

var pvcUnboundError = &microerror.Error{
	Kind: "pvcUnboundError",
}

var podExecError = &microerror.Error{
	Kind: "podExecError",
}

var prometheusQueryError = &microerror.Error{
	Kind: "prometheusQueryError",
}

var targetDownError = &microerror.Error{
	Kind: "targetDownError",
}

var unexpectedAnswerError = &microerror.Error{
	Kind: "unexpectedAnswerError",
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
