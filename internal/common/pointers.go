package common

func StringToPtr(s string) *string {
	return &s
}

func BoolToPtr(b bool) *bool {
	return &b
}

func ToPtr[T any](x T) *T {
	return &x
}
