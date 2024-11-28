package api_test

import (
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/tanjd/core-repository/apps/identity/api"
	"github.com/tanjd/core-repository/apps/identity/handler"
	"github.com/tanjd/core-repository/apps/identity/repo"
)

func TestHealthCheckEndpoint(t *testing.T) {
	_, a := humatest.New(t)

	r := repo.NewInMemoryRepo()
	userHandler := handler.NewUserHandler(r)

	routes := api.NewRouter(userHandler, a)
	routes.AddHealthCheckRoutes()

	t.Run("Get health check endpoint ", func(t *testing.T) {
		resp := a.Get("/health")

		assert.Equal(t, 200, resp.Result().StatusCode)
		assert.Contains(t, resp.Body.String(), `"status":"OK"`)
	})
}
