package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type pinger struct{ err error }

func (p pinger) Ping(context.Context) error { return p.err }

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err  error
		code int
		body string
	}{{nil, http.StatusOK, "ok"}, {errors.New("down"), http.StatusServiceUnavailable, "database unavailable"}} {
		r := gin.New()
		r.GET("/healthz", health(pinger{tc.err}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		assert.Equal(t, tc.code, w.Code)
		assert.Equal(t, tc.body, w.Body.String())
	}
}
