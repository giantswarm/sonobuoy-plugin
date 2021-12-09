package capiutil

import "github.com/giantswarm/microerror"

var labelNotSetError = &microerror.Error{
	Kind: "labelNotSetError",
}

// IsLabelNotSet asserts labelNotSetError.
func IsLabelNotSet(err error) bool {
	return microerror.Cause(err) == labelNotSetError
}

var notFoundError = &microerror.Error{
	Kind: "notFoundError",
}

// IsNotFound asserts notFoundError.
func IsNotFound(err error) bool {
	return microerror.Cause(err) == notFoundError
}

var tooManyObjectsError = &microerror.Error{
	Kind: "tooManyObjects",
}

// IsTooManyObjectsError asserts tooManyObjectsError.
func IsTooManyObjectsError(err error) bool {
	return microerror.Cause(err) == tooManyObjectsError
}

var unexpectedConditionValueError = &microerror.Error{
	Kind: "unexpectedConditionValue",
}

// IsUnexpectedConditionValue asserts unexpectedConditionValueError.
func IsUnexpectedConditionValue(err error) bool {
	return microerror.Cause(err) == unexpectedConditionValueError
}
