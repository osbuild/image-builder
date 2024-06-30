package oauth2

import "context"

// DummyToken is a dummy implementation of the Tokener interface.
type DummyToken struct{}

// Token returns a static "testtoken" string.
func (dt *DummyToken) Token(ctx context.Context) (string, error) {

	return "accesstoken", nil
}

func (dt *DummyToken) ForceRefresh(ctx context.Context) (string, error) {
	return "", nil
}
