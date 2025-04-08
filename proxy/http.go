// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/encoding"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/transport/http/client"
)

var httpProxy = CustomHTTPProxyFactory(client.NewHTTPClient)

// HTTPProxyFactory returns a BackendFactory. The Proxies it creates will use the received net/http.Client
func HTTPProxyFactory(client *http.Client) BackendFactory {
	return CustomHTTPProxyFactory(func(_ context.Context) *http.Client { return client })
}

// CustomHTTPProxyFactory returns a BackendFactory. The Proxies it creates will use the received HTTPClientFactory
func CustomHTTPProxyFactory(cf client.HTTPClientFactory) BackendFactory {
	return func(backend *config.Backend) Proxy {
		return NewHTTPProxy(backend, cf, backend.Decoder)
	}
}

// NewHTTPProxy creates a http proxy with the injected configuration, HTTPClientFactory and Decoder
func NewHTTPProxy(remote *config.Backend, cf client.HTTPClientFactory, decode encoding.Decoder) Proxy {
	return NewHTTPProxyWithHTTPExecutor(remote, client.DefaultHTTPRequestExecutor(cf), decode)
}

// NewHTTPProxyWithHTTPExecutor creates a http proxy with the injected configuration, HTTPRequestExecutor and Decoder
func NewHTTPProxyWithHTTPExecutor(remote *config.Backend, re client.HTTPRequestExecutor, dec encoding.Decoder) Proxy {
	if remote.Encoding == encoding.NOOP {
		return NewHTTPProxyDetailed(remote, re, client.NoOpHTTPStatusHandler, NoOpHTTPResponseParser)
	}

	ef := NewEntityFormatter(remote)
	rp := DefaultHTTPResponseParserFactory(HTTPResponseParserConfig{dec, ef})
	return NewHTTPProxyDetailed(remote, re, client.GetHTTPStatusHandler(remote), rp)
}

// NewHTTPProxyDetailed creates a http proxy with the injected configuration, HTTPRequestExecutor,
// Decoder and HTTPResponseParser
func NewHTTPProxyDetailed(config *config.Backend, re client.HTTPRequestExecutor, ch client.HTTPStatusHandler, rp HTTPResponseParser) Proxy {
	var wsProxy *httputil.ReverseProxy
	log.Printf("METHOD: %s", config.Method)
	if strings.ToLower(config.Method) == "ws" || strings.ToLower(config.Method) == "wss" {
		jsonConfig, _ := json.Marshal(config)
		log.Println("config", string(jsonConfig))
		backendURL, err := url.Parse(config.Host[0])
		if err != nil {
			log.Printf("Erro ao parsear URL do backend: %v", err)
			return nil
		}

		// Configuração do proxy reverso para WebSocket
		wsProxy = httputil.NewSingleHostReverseProxy(backendURL)
		wsProxy.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", backendURL.Host)
			},
		}
	}

	return func(ctx context.Context, request *Request, responseWriter http.ResponseWriter, requestContext *http.Request) (*Response, error) {
		log.Printf("aqui")
		if wsProxy != nil {
			fmt.Printf("URL: %s, PATH: %s", request.URL.Host, request.URL.Path)
			originalDirector := wsProxy.Director
			wsProxy.Director = func(req *http.Request) {
				originalDirector(req)
				req.URL.Scheme = "http"
				req.URL.Host = request.URL.Host
				req.URL.Path = request.Path
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Upgrade", "websocket")
				for k, vs := range request.Headers {
					tmp := make([]string, len(vs))
					copy(tmp, vs)
					req.Header.Set(k, tmp[0])
				}
			}

			wsProxy.ServeHTTP(responseWriter, requestContext)
			return nil, nil

		} else {
			requestToBackend, err := http.NewRequest(strings.ToTitle(request.Method), request.URL.String(), request.Body)
			if err != nil {
				return nil, err
			}
			requestToBackend.Header = make(map[string][]string, len(request.Headers))
			for k, vs := range request.Headers {
				tmp := make([]string, len(vs))
				copy(tmp, vs)
				requestToBackend.Header[k] = tmp
			}
			if request.Body != nil {
				if v, ok := request.Headers["Content-Length"]; ok && len(v) == 1 && v[0] != "chunked" {
					if size, err := strconv.Atoi(v[0]); err == nil {
						requestToBackend.ContentLength = int64(size)
					}
				}
			}

			resp, err := re(ctx, requestToBackend)
			if requestToBackend.Body != nil {
				requestToBackend.Body.Close()
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if err != nil {
				return nil, err
			}

			resp, err = ch(ctx, resp)
			if err != nil {
				if t, ok := err.(responseError); ok {
					return &Response{
						Data: map[string]interface{}{
							fmt.Sprintf("error_%s", t.Name()): t,
						},
						Metadata: Metadata{StatusCode: t.StatusCode()},
					}, nil
				}
				return nil, err
			}

			return rp(ctx, resp)
		}
	}
}

// NewRequestBuilderMiddleware creates a proxy middleware that parses the request params received
// from the outer layer and generates the path to the backend endpoints
var NewRequestBuilderMiddleware = func(remote *config.Backend) Middleware {
	return newRequestBuilderMiddleware(logging.NoOp, remote)
}

func NewRequestBuilderMiddlewareWithLogger(logger logging.Logger, remote *config.Backend) Middleware {
	return newRequestBuilderMiddleware(logger, remote)
}

func newRequestBuilderMiddleware(l logging.Logger, remote *config.Backend) Middleware {
	return func(next ...Proxy) Proxy {
		if len(next) > 1 {
			l.Fatal("too many proxies for this %s %s -> %s proxy middleware: newRequestBuilderMiddleware only accepts 1 proxy, got %d", remote.ParentEndpointMethod, remote.ParentEndpoint, remote.URLPattern, len(next))
			return nil
		}
		return func(ctx context.Context, r *Request, responseWriter http.ResponseWriter, requestContext *http.Request) (*Response, error) {
			r.GeneratePath(remote.URLPattern)
			r.Method = remote.Method
			return next[0](ctx, r, responseWriter, requestContext)
		}
	}
}

type responseError interface {
	Error() string
	Name() string
	StatusCode() int
}
