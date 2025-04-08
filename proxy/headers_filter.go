// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
)

// NewFilterHeadersMiddleware returns a middleware with or without a header filtering
// proxy wrapping the next element (depending on the configuration).
func NewFilterHeadersMiddleware(logger logging.Logger, remote *config.Backend) Middleware {
	if len(remote.HeadersToPass) == 0 {
		return emptyMiddlewareFallback(logger)
	}

	return func(next ...Proxy) Proxy {
		if len(next) > 1 {
			logger.Fatal("too many proxies for this %s %s -> %s proxy middleware: NewFilterHeadersMiddleware only accepts 1 proxy, got %d", remote.ParentEndpointMethod, remote.ParentEndpoint, remote.URLPattern, len(next))
			return nil
		}
		nextProxy := next[0]
		return func(ctx context.Context, request *Request, responseWriter http.ResponseWriter, requestContext *http.Request) (*Response, error) {
			if len(request.Headers) == 0 {
				return nextProxy(ctx, request, responseWriter, requestContext)
			}
			numHeadersToPass := 0
			for _, v := range remote.HeadersToPass {
				if _, ok := request.Headers[v]; ok {
					numHeadersToPass++
				}
			}
			if numHeadersToPass == len(request.Headers) {
				// all the headers should pass, no need to clone the headers
				return nextProxy(ctx, request, responseWriter, requestContext)
			}
			// ATTENTION: this is not a clone of headers!
			// this just filters the headers we do not want to send:
			// issues and race conditions could happen the same way as when we
			// do not filter the headers. This is a design decission, and if we
			// want to clone the header values (because of write modifications),
			// that should be done at an upper level (so the approach is the same
			// for non filtered parallel requests).
			newHeaders := make(map[string][]string, numHeadersToPass)
			for _, v := range remote.HeadersToPass {
				if values, ok := request.Headers[v]; ok {
					newHeaders[v] = values
				}
			}
			return nextProxy(ctx, &Request{
				Method:  request.Method,
				URL:     request.URL,
				Query:   request.Query,
				Path:    request.Path,
				Body:    request.Body,
				Params:  request.Params,
				Headers: newHeaders,
			}, responseWriter, requestContext)
		}
	}
}
