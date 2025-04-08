// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/luraproject/lura/v2/logging"
)

// NewLoggingMiddleware creates proxy middleware for logging requests and responses
func NewLoggingMiddleware(logger logging.Logger, name string) Middleware {
	logPrefix := "[" + strings.ToUpper(name) + "]"
	return func(next ...Proxy) Proxy {
		if len(next) > 1 {
			logger.Fatal("too many proxies for this proxy middleware: NewLoggingMiddleware only accepts 1 proxy, got %d", len(next))
			return nil
		}
		return func(ctx context.Context, request *Request, responseWriter http.ResponseWriter, requestContext *http.Request) (*Response, error) {
			begin := time.Now()
			logger.Info(logPrefix, "Calling backend")
			logger.Debug(logPrefix, "Request", request)

			result, err := next[0](ctx, request, responseWriter, requestContext)

			logger.Info(logPrefix, "Call to backend took", time.Since(begin).String())
			if err != nil {
				logger.Warning(logPrefix, "Call to backend failed:", err.Error())
				return result, err
			}
			if result == nil {
				logger.Warning(logPrefix, "Call to backend returned a null response")
			}

			return result, err
		}
	}
}
