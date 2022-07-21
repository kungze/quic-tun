package options

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.ClientOptions.Validate()...)
	errs = append(errs, o.RestfulAPIOptions.Validate()...)
	errs = append(errs, o.LogOptions.Validate()...)

	return errs
}
