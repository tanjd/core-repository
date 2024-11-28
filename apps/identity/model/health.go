package model

type HealthCheckResponse struct {
	Body struct {
		Status string `json:"status" doc:"Status of the service"`
		Uptime string `json:"uptime" doc:"Time the service has been running"`
	}
}
