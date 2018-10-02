package recordset

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/giantswarm/microerror"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var noUpdateError = &microerror.Error{
	Kind: "noUpdateError",
}

// IsNoUpdateError asserts noUpdateError.
func IsNoUpdateError(err error) bool {
	if microerror.Cause(err) == noUpdateError {
		return true
	}

	awsErr, ok := err.(awserr.Error)
	return ok &&
		awsErr.Code() == "ValidationError" &&
		awsErr.Message() == "No updates are to be performed."
}

var tooFewResultsError = &microerror.Error{
	Kind: "tooFewResultsError",
}

// IsTooFewResults asserts tooFewResultsError.
func IsTooFewResults(err error) bool {
	return microerror.Cause(err) == tooFewResultsError
}
