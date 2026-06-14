package slugerrors

type ErrorType struct {
	name string
}

var (
	ErrorTypeBadRequest = ErrorType{"bad-request"}
	ErrorTypeNotFound   = ErrorType{"not-found"}
	ErrorTypeInternal   = ErrorType{"internal"}
)

type SlugError struct {
	error     string
	slug      string
	errorType ErrorType
}

func (e SlugError) Error() string {
	return e.error
}

func (e SlugError) Slug() string {
	return e.slug
}

func (e SlugError) ErrorType() ErrorType {
	return e.errorType
}

func NewBadRequestError(error string, slug string) SlugError {
	return SlugError{error: error, slug: slug, errorType: ErrorTypeBadRequest}
}

func NewNotFoundError(error string, slug string) SlugError {
	return SlugError{error: error, slug: slug, errorType: ErrorTypeNotFound}
}
