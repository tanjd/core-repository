package api_test

import (
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/tanjd/core-repository/apps/identity/api"
	"github.com/tanjd/core-repository/apps/identity/handler"
)

func TestHealthCheckEndpoint(t *testing.T) {
	_, a := humatest.New(t)

	routes := api.NewRouter(handler.UserHandler{}, &handler.AuthenticationHandler{}, a)
	routes.AddHealthCheckRoutes()

	t.Run("Get health check endpoint ", func(t *testing.T) {
		resp := a.Get("/health")

		assert.Equal(t, 200, resp.Result().StatusCode)
		assert.Contains(t, resp.Body.String(), `"status":"OK"`)
	})
}
