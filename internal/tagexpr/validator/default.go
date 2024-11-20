package validator

var defaultValidator = New("vd").SetErrorFactory(defaultErrorFactory)

// Default returns the default validator.
// NOTE:
//  The tag name is 'vd'
func Default() *Validator {
	return defaultValidator
}

// Validate uses the default validator to validate whether the fields of value is valid.
// NOTE:
//  The tag name is 'vd'
//  If checkAll=true, validate all the error.
func Validate(value interface{}, checkAll ...bool) error {
	return defaultValidator.Validate(value, checkAll...)
}

// SetErrorFactory customizes the factory of validation error for the default validator.
// NOTE:
//  The tag name is 'vd'
func SetErrorFactory(errFactory func(fieldSelector, msg string) error) {
	defaultValidator.SetErrorFactory(errFactory)
}
