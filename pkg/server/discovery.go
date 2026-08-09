package server

import (
	"context"
	"log/slog"

	commonDiscovery "github.com/saker-ai/saker-common/discovery"
	"github.com/saker-ai/skillhub/pkg/config"
)

func startServiceDiscovery(ctx context.Context, logger *slog.Logger, cfg *config.Config) *commonDiscovery.MultiRegistration {
	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	prefix := cfg.Server.BasePath
	if prefix == "" {
		prefix = "/skillhub"
	}
	reg, err := commonDiscovery.StartFromEnv(ctx, commonDiscovery.ServiceInstance{
		ID: "skillhub", Name: "SkillHub", Scheme: "http", Address: host, Port: cfg.Server.Port,
		Prefix: prefix, HealthPath: "/healthz", Audience: "skillhub", NativeRoute: "/skills",
	}, commonDiscovery.EnvOptions{})
	if err != nil {
		logger.WarnContext(ctx, "skillhub discovery registration failed", "error", err)
		return nil
	}
	return reg
}
