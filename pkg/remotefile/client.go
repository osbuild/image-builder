package remotefile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	doer Doer
}

func NewClient(doer Doer) *Client {
	if doer == nil {
		doer = &http.Client{}
	}

	return &Client{
		doer: doer,
	}
}

func (c *Client) makeRequest(ctx context.Context, u *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, u.String())
	}

	output, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (c *Client) validateURL(u string) (*url.URL, error) {
	if u == "" {
		return nil, fmt.Errorf("File resolver: url is required")
	}
	parsedURL, err := url.ParseRequestURI(u)
	if err != nil {
		return nil, fmt.Errorf("File resolver: invalid url %s", u)
	}
	return parsedURL, nil
}

// resolve and return the contents of a remote file
// which can be used later, in the pipeline
func (c *Client) Resolve(ctx context.Context, u string) ([]byte, error) {
	parsedURL, err := c.validateURL(u)
	if err != nil {
		return nil, err
	}

	return c.makeRequest(ctx, parsedURL)
}
