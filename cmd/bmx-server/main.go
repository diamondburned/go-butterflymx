//go:build goexperiment.jsonv2

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
)

// Options for the CLI configuration.
type Options struct {
	Host         string `help:"Host to listen on" default:"0.0.0.0" env:"HOST"`
	Port         int    `help:"Port to listen on" default:"8888" env:"PORT"`
	APISecret    string `help:"API secret to authenticate requests" env:"API_SECRET"`
	TLSCertPath  string `help:"Path to TLS certificate file" env:"TLS_CERT_PATH"`
	TLSKeyPath   string `help:"Path to TLS private key file" env:"TLS_KEY_PATH"`
	ClientCAPath string `help:"Path to Client CA certificate for mTLS" env:"CLIENT_CA_PATH"`
	EnableMTLS   bool   `help:"Enable mutual TLS" env:"ENABLE_MTLS"`
	DefaultToken string `help:"Default ButterflyMX API token" env:"BUTTERFLYMX_API_TOKEN"`
}

func main() {
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		r := chi.NewRouter()
		r.Use(AuthMiddleware(options.APISecret))
		r.Use(ContextMiddleware(options.DefaultToken))

		config := huma.DefaultConfig("ButterflyMX Proxy Server", "1.0.0")
		api := humachi.New(r, config)

		registerRoutes(api)

		hooks.OnStart(func() {
			addr := fmt.Sprintf("%s:%d", options.Host, options.Port)
			server := &http.Server{
				Addr:    addr,
				Handler: r,
			}

			if options.EnableMTLS || options.ClientCAPath != "" {
				tlsConfig := &tls.Config{}
				if options.ClientCAPath != "" {
					caCert, err := os.ReadFile(options.ClientCAPath)
					if err != nil {
						log.Fatalf("failed to read client CA cert: %v", err)
					}
					caCertPool := x509.NewCertPool()
					if !caCertPool.AppendCertsFromPEM(caCert) {
						log.Fatalf("failed to append client CA cert to pool")
					}
					tlsConfig.ClientCAs = caCertPool
					tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
				} else {
					tlsConfig.ClientAuth = tls.RequireAnyClientCert
				}
				server.TLSConfig = tlsConfig
			}

			if options.TLSCertPath != "" && options.TLSKeyPath != "" {
				log.Printf("Starting TLS server on https://%s", addr)
				if err := server.ListenAndServeTLS(options.TLSCertPath, options.TLSKeyPath); err != nil && err != http.ErrServerClosed {
					log.Fatalf("TLS Server failed: %v", err)
				}
			} else {
				log.Printf("Starting HTTP server on http://%s", addr)
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("HTTP Server failed: %v", err)
				}
			}
		})
	})

	cli.Run()
}

func registerRoutes(api huma.API) {
	registerTenantRoutes(api)
	registerKeychainRoutes(api)
}
