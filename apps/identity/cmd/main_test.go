package main

import (
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheckEndpoint(t *testing.T) {
	_, api := humatest.New(t)

	addRoutes(api)
	t.Run("Get health check endpoint ", func(t *testing.T) {
		resp := api.Get("/health")

		assert.Equal(t, 200, resp.Result().StatusCode)
		assert.Contains(t, resp.Body.String(), `"status":"OK"`)
	})
}
