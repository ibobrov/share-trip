package main

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

func (s *stack) environment(app string) (map[string]string, error) {
	values, err := godotenv.Read(filepath.Join(s.projects[app], ".env"))
	if err != nil {
		return nil, fmt.Errorf("read %s .env: file missing or invalid", app)
	}
	if raw := values["DATABASE_DSN"]; raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.User == nil {
			return nil, fmt.Errorf("%s DATABASE_DSN must be a PostgreSQL URL", app)
		}
		values["POSTGRES_USER"] = u.User.Username()
		values["POSTGRES_PASSWORD"], _ = u.User.Password()
		values["POSTGRES_DB"] = strings.TrimPrefix(u.Path, "/")
	}
	for _, key := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"} {
		if values[key] == "" {
			return nil, fmt.Errorf("%s .env: %s is required", app, key)
		}
	}
	return values, nil
}

func (s *stack) secret(name string, data map[string]string) error {
	object := resource("Secret", name)
	object["type"], object["stringData"] = "Opaque", data
	return s.applyObject(object)
}

func (s *stack) secrets() error {
	root, err := s.environment("sharetrip")
	if err != nil {
		return err
	}
	if root["KEYCLOAK_CLIENT_SECRET"] == "" || root["POSTGRES_EXPORTER_PASSWORD"] == "" {
		return errors.New("sharetrip .env requires KEYCLOAK_CLIENT_SECRET and POSTGRES_EXPORTER_PASSWORD")
	}
	for _, app := range apps {
		env, err := s.environment(app)
		if err != nil {
			return err
		}
		pg := map[string]string{}
		for _, key := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"} {
			pg[key] = env[key]
		}
		if err := s.secret("postgres-"+app+"-secret", pg); err != nil {
			return err
		}
		dsn := (&url.URL{Scheme: "postgres", User: url.UserPassword(env["POSTGRES_USER"], env["POSTGRES_PASSWORD"]), Host: "postgres-" + app + ":5432", Path: "/" + env["POSTGRES_DB"], RawQuery: "sslmode=disable"}).String()
		secret := map[string]string{"DATABASE_DSN": dsn}
		if app == "sharetrip" {
			secret["KEYCLOAK_CLIENT_SECRET"] = root["KEYCLOAK_CLIENT_SECRET"]
		}
		if err := s.secret(app+"-secret", secret); err != nil {
			return err
		}
		if err := s.secret("postgres-exporter-"+app+"-secret", map[string]string{
			"DATA_SOURCE_URI":  "postgres-" + app + ":5432/" + env["POSTGRES_DB"] + "?sslmode=disable",
			"DATA_SOURCE_USER": "postgres_exporter", "DATA_SOURCE_PASS": root["POSTGRES_EXPORTER_PASSWORD"],
		}); err != nil {
			return err
		}
	}
	if err := s.secret("monitoring-secret", map[string]string{"POSTGRES_EXPORTER_PASSWORD": root["POSTGRES_EXPORTER_PASSWORD"]}); err != nil {
		return err
	}
	// Only used when there is no existing Keycloak data. Existing users are preserved.
	return s.secret("keycloak-secret", map[string]string{"KC_BOOTSTRAP_ADMIN_USERNAME": "admin", "KC_BOOTSTRAP_ADMIN_PASSWORD": "admin"})
}

func (s *stack) configs() error {
	for name, files := range map[string]map[string]string{
		"loki-config":         {"config.yaml": "deploy/loki/config.yaml"},
		"otel-config":         {"config.yaml": "deploy/otel-collector/config.yaml"},
		"alloy-config":        {"config.alloy": "k8s/config/config.alloy"},
		"prometheus-config":   {"prometheus.yml": "k8s/config/prometheus.yml"},
		"grafana-datasources": {"local.yaml": "k8s/config/datasources.yaml"},
		"exporter-init":       {"init.sql": "deploy/postgres/init-exporter.sql", "init.sh": "k8s/config/init-exporter.sh"},
		"keycloak-realm":      {"sharetrip-realm.json": "k8s/config/sharetrip-realm.json"},
	} {
		for key, path := range files {
			files[key] = filepath.Join(s.root, path)
		}
		if err := s.configMap(name, files); err != nil {
			return err
		}
	}
	return nil
}
