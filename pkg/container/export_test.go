package container

import "github.com/containers/image/v5/types"

func NewResolverWithTestClient(arch string, f func(string) (*Client, error)) *asyncResolver {
	resolver := NewResolver(arch)
	resolver.newClient = f
	return resolver
}

func NewClientWithTestStorage(target, storage string) (*Client, error) {
	client, err := NewClient(target)
	client.store = storage
	return client, err
}

func NewBlockingResolverWithTestClient(arch string, f func(string) (*Client, error)) Resolver {
	resolver := NewBlockingResolver(arch)
	resolver.(*blockingResolver).newClient = f
	return resolver
}

func ClientSysctx(cl *Client) *types.SystemContext {
	return cl.sysCtx
}

var ParseImageName = parseImageName
