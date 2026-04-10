package remotefile

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
)

type resolveResult struct {
	url     string
	content []byte
	err     error
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type ResolverOption func(*resolverOptions)

type resolverOptions struct {
	doer Doer
}

func WithDoer(doer Doer) ResolverOption {
	return func(opts *resolverOptions) {
		opts.doer = doer
	}
}

// TODO: could make this more generic since this is shared with the container
// resolver
type Resolver struct {
	results chan resolveResult
	wg      sync.WaitGroup
	ctx     context.Context
	client  *Client
}

func NewResolver(ctx context.Context, opts ...ResolverOption) *Resolver {
	options := &resolverOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return &Resolver{
		results: make(chan resolveResult),
		wg:      sync.WaitGroup{},
		ctx:     ctx,
		client:  NewClient(options.doer),
	}
}

// Add a URL to the resolver queue. When called after Finish was called,
// it may panic.
func (r *Resolver) Add(url string) {
	r.wg.Add(1)

	go func() {
		defer r.wg.Done()

		content, err := r.client.Resolve(r.ctx, url)
		r.results <- resolveResult{url: url, content: content, err: err}
	}()
}

// Finish starts collecting of results and returns them. No further calls to Add
// are allowed after this call. It blocks until all results are collected.
func (r *Resolver) Finish() ([]Spec, error) {
	go func() {
		r.wg.Wait()
		close(r.results)
	}()

	var resultItems []Spec
	var errs []string
	for result := range r.results {
		if result.err == nil {
			resultItems = append(resultItems, Spec{URL: result.url, Content: result.content})
		} else {
			errs = append(errs, result.err.Error())
		}
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		return resultItems, fmt.Errorf("failed to resolve remote files: %s", strings.Join(slices.Compact(errs), "; "))
	}

	return resultItems, nil
}
