package handler

import (
	"context"
	"time"

	"github.com/tanjd/core-repository/apps/identity/model"
)

var startTime = time.Now()

func HealthCheckHandler(ctx context.Context, request *struct{}) (*model.HealthCheckResponse, error) {
	resp := &model.HealthCheckResponse{}
	resp.Body.Status = "OK"
	resp.Body.Uptime = time.Since(startTime).String()

	return resp, nil
}
