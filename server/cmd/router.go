package main

import (
	"fmt"

	"expenser/internal/config"

	"github.com/gin-gonic/gin"
)

// trustedProxies are the private networks a reverse proxy can be on.
var trustedProxies = []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}

// newRouter makes the Gin engine. The mode is set from the config, so it must
// run after the config is loaded: Gin's debug mode logs every route and warns
// on startup, which production should not do.
func newRouter(cfg *config.Config) (*gin.Engine, error) {
	if cfg.DevMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	// Only nginx (on the Docker network) and local tools may set
	// X-Forwarded-For; anyone else's is ignored.
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}
	return router, nil
}
