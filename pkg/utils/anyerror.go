package utils

// AnyError returns an error if any given error is not nil.
func AnyError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
