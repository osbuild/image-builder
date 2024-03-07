package common

func ToPtr[T any](x T) *T {
	return &x
}

func FromPtr[T any](ref *T) (value T) {
	if ref != nil {
		value = *ref
	}
	return
}
