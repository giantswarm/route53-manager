package recordset

import "github.com/giantswarm/microerror"

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var tooFewResultsError = &microerror.Error{
	Kind: "tooFewResultsError",
}

// IsTooFewResults asserts tooFewResultsError.
func IsTooFewResults(err error) bool {
	return microerror.Cause(err) == tooFewResultsError
}
