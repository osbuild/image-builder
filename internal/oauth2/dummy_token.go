package oauth2

import "context"

// DummyToken is a dummy implementation of the Tokener interface.
type DummyToken struct{}

// NextToken returns a static "testtoken" string.
func (dt *DummyToken) NextToken(ctx context.Context) (string, error) {

	return "testtoken", nil
}

func (dt *DummyToken) ForceRefresh(ctx context.Context) (string, error) {
	return "", nil
}
