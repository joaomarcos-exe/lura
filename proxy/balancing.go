// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"net/http"
	"net/url"

	"github.com/joaomarcos-exe/lura/v2/config"
	"github.com/joaomarcos-exe/lura/v2/logging"
	"github.com/joaomarcos-exe/lura/v2/sd"
)

// NewLoadBalancedMiddleware creates proxy middleware adding the most perfomant balancer
// over a default subscriber
func NewLoadBalancedMiddleware(remote *config.Backend) Middleware {
	return NewLoadBalancedMiddlewareWithSubscriber(sd.GetRegister().Get(remote.SD)(remote))
}

// NewLoadBalancedMiddlewareWithSubscriber creates proxy middleware adding the most perfomant balancer
// over the received subscriber
func NewLoadBalancedMiddlewareWithSubscriber(subscriber sd.Subscriber) Middleware {
	return newLoadBalancedMiddleware(logging.NoOp, sd.NewBalancer(subscriber))
}

// NewRoundRobinLoadBalancedMiddleware creates proxy middleware adding a round robin balancer
// over a default subscriber
func NewRoundRobinLoadBalancedMiddleware(remote *config.Backend) Middleware {
	return NewRoundRobinLoadBalancedMiddlewareWithSubscriber(sd.GetRegister().Get(remote.SD)(remote))
}

// NewRandomLoadBalancedMiddleware creates proxy middleware adding a random balancer
// over a default subscriber
func NewRandomLoadBalancedMiddleware(remote *config.Backend) Middleware {
	return NewRandomLoadBalancedMiddlewareWithSubscriber(sd.GetRegister().Get(remote.SD)(remote))
}

// NewRoundRobinLoadBalancedMiddlewareWithSubscriber creates proxy middleware adding a round robin
// balancer over the received subscriber
func NewRoundRobinLoadBalancedMiddlewareWithSubscriber(subscriber sd.Subscriber) Middleware {
	return newLoadBalancedMiddleware(logging.NoOp, sd.NewRoundRobinLB(subscriber))
}

// NewRandomLoadBalancedMiddlewareWithSubscriber creates proxy middleware adding a random
// balancer over the received subscriber
func NewRandomLoadBalancedMiddlewareWithSubscriber(subscriber sd.Subscriber) Middleware {
	return newLoadBalancedMiddleware(logging.NoOp, sd.NewRandomLB(subscriber))
}

// NewLoadBalancedMiddlewareWithLogger creates proxy middleware adding the most perfomant balancer
// over a default subscriber
func NewLoadBalancedMiddlewareWithLogger(l logging.Logger, remote *config.Backend) Middleware {
	return NewLoadBalancedMiddlewareWithSubscriberAndLogger(l, sd.GetRegister().Get(remote.SD)(remote))
}

// NewLoadBalancedMiddlewareWithSubscriberAndLogger creates proxy middleware adding the most perfomant balancer
// over the received subscriber
func NewLoadBalancedMiddlewareWithSubscriberAndLogger(l logging.Logger, subscriber sd.Subscriber) Middleware {
	return newLoadBalancedMiddleware(l, sd.NewBalancer(subscriber))
}

// NewRoundRobinLoadBalancedMiddlewareWithLogger creates proxy middleware adding a round robin balancer
// over a default subscriber
func NewRoundRobinLoadBalancedMiddlewareWithLogger(l logging.Logger, remote *config.Backend) Middleware {
	return NewRoundRobinLoadBalancedMiddlewareWithSubscriberAndLogger(l, sd.GetRegister().Get(remote.SD)(remote))
}

// NewRandomLoadBalancedMiddlewareWithLogger creates proxy middleware adding a random balancer
// over a default subscriber
func NewRandomLoadBalancedMiddlewareWithLogger(l logging.Logger, remote *config.Backend) Middleware {
	return NewRandomLoadBalancedMiddlewareWithSubscriberAndLogger(l, sd.GetRegister().Get(remote.SD)(remote))
}

// NewRoundRobinLoadBalancedMiddlewareWithSubscriberAndLogger creates proxy middleware adding a round robin
// balancer over the received subscriber
func NewRoundRobinLoadBalancedMiddlewareWithSubscriberAndLogger(l logging.Logger, subscriber sd.Subscriber) Middleware {
	return newLoadBalancedMiddleware(l, sd.NewRoundRobinLB(subscriber))
}

// NewRandomLoadBalancedMiddlewareWithSubscriberAndLogger creates proxy middleware adding a random
// balancer over the received subscriber
func NewRandomLoadBalancedMiddlewareWithSubscriberAndLogger(l logging.Logger, subscriber sd.Subscriber) Middleware {
	return newLoadBalancedMiddleware(l, sd.NewRandomLB(subscriber))
}

func newLoadBalancedMiddleware(l logging.Logger, lb sd.Balancer) Middleware {
	return func(next ...Proxy) Proxy {
		if len(next) > 1 {
			l.Fatal("too many proxies for this proxy middleware: newLoadBalancedMiddleware only accepts 1 proxy, got %d", len(next))
			return nil
		}
		return func(ctx context.Context, r *Request, responseWriter http.ResponseWriter, requestContext *http.Request) (*Response, error) {
			host, err := lb.Host()
			if err != nil {
				return nil, err
			}

			r.URL, err = url.Parse(host + r.Path)
			if err != nil {
				return nil, err
			}
			if len(r.Query) > 0 {
				if len(r.URL.RawQuery) > 0 {
					r.URL.RawQuery += "&" + r.Query.Encode()
				} else {
					r.URL.RawQuery += r.Query.Encode()
				}
			}

			return next[0](ctx, r, responseWriter, requestContext)
		}
	}
}
