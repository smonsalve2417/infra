package discovery

import "context"

type Registry interface {
	Register(ctx context.Context, instanceID string, serverName string, hostPort string) error
	DeRegister(ctx context.Context, instanceID string, serverName string) error
	Discover(ctx context.Context, serviceName string) ([]string, error)
	HealthCheck(instanceID string, serviceName string) error
}
