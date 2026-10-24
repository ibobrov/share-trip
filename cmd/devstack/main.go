// devstack runs this project's local stack on Docker Desktop Kubernetes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const namespace = "sharetrip"

var apps = []string{"sharetrip", "contract", "notification"}

type stack struct {
	ctx         context.Context
	root, local string
	projects    map[string]string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		return errors.New("usage: go run ./cmd/devstack up|down")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	s := &stack{ctx: ctx, root: root, local: filepath.Join(root, ".local", "k8s"), projects: map[string]string{
		"sharetrip":    root,
		"contract":     projectPath("CONTRACT_PROJECT_DIR", filepath.Join(home, "GolandProjects", "share-trip-contract")),
		"notification": projectPath("NOTIFICATION_PROJECT_DIR", filepath.Join(home, "GolandProjects", "share-trip-notification")),
	}}
	if err := os.MkdirAll(s.local, 0700); err != nil {
		return err
	}
	if err := s.kube("get", "node", "desktop-control-plane"); err != nil {
		return err
	}
	if os.Args[1] == "down" {
		if err := s.kube("-n", namespace, "scale", "deployment,statefulset", "--all", "--replicas=0"); err != nil {
			return err
		}
		return s.kube("-n", namespace, "wait", "--for=delete", "pod", "-l", "app", "--timeout=2m")
	}
	return s.up()
}

func projectPath(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (s *stack) command(name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(s.ctx, name, args...)
	cmd.Dir = s.root
	return cmd
}

func execute(cmd *exec.Cmd) error {
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", filepath.Base(cmd.Path), err)
	}
	return nil
}

func (s *stack) kube(args ...string) error {
	return execute(s.command("kubectl", append([]string{"--context", "docker-desktop"}, args...)...))
}

func (s *stack) docker(args ...string) error {
	return execute(s.command("docker", append([]string{"--context", "desktop-linux"}, args...)...))
}

func (s *stack) capture(name string, args ...string) ([]byte, error) {
	cmd := s.command(name, args...)
	// Captured commands may return secrets. Never echo their output on failure.
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return data, nil
}

func (s *stack) applyObject(object any) error {
	data, err := json.Marshal(object)
	if err != nil {
		return err
	}
	cmd := s.command("kubectl", "--context", "docker-desktop", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(string(data))
	return execute(cmd)
}

func (s *stack) apply(path string) error { return s.kube("apply", "-f", filepath.Join(s.root, path)) }

func resource(kind, name string) map[string]any {
	return map[string]any{"apiVersion": "v1", "kind": kind, "metadata": map[string]string{"name": name, "namespace": namespace}}
}

func (s *stack) configMap(name string, files map[string]string) error {
	data := make(map[string]string, len(files))
	for key, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data[key] = string(content)
	}
	object := resource("ConfigMap", name)
	object["data"] = data
	return s.applyObject(object)
}

func (s *stack) wait(kind, name string) error {
	return s.kube("-n", namespace, "rollout", "status", kind+"/"+name, "--timeout=5m")
}

func (s *stack) up() error {
	for _, app := range apps {
		if _, err := os.Stat(filepath.Join(s.projects[app], "go.mod")); err != nil {
			return fmt.Errorf("%s project: %w", app, err)
		}
		if _, err := s.environment(app); err != nil {
			return err
		}
	}
	rootEnv, err := s.environment("sharetrip")
	if err != nil {
		return err
	}
	if rootEnv["KEYCLOAK_CLIENT_SECRET"] == "" || rootEnv["POSTGRES_EXPORTER_PASSWORD"] == "" {
		return errors.New("sharetrip .env requires KEYCLOAK_CLIENT_SECRET and POSTGRES_EXPORTER_PASSWORD")
	}
	if err := s.build(); err != nil {
		return err
	}
	if err := s.apply("k8s/namespace.yaml"); err != nil {
		return err
	}
	if err := s.checkCluster(); err != nil {
		return err
	}
	if err := s.migrateCompose(); err != nil {
		return err
	}
	if err := s.secrets(); err != nil {
		return err
	}
	if err := s.configs(); err != nil {
		return err
	}
	if err := s.apply("k8s/infra/postgres.yaml"); err != nil {
		return err
	}
	for _, app := range apps {
		if err := s.wait("statefulset", "postgres-"+app); err != nil {
			return err
		}
		if err := s.exporterJob(app); err != nil {
			return err
		}
	}
	for _, path := range []string{"messaging-auth.yaml", "observability.yaml"} {
		if err := s.apply("k8s/infra/" + path); err != nil {
			return err
		}
	}
	for _, name := range []string{"kafka", "keycloak", "grafana", "loki", "prometheus"} {
		if err := s.wait("statefulset", name); err != nil {
			return err
		}
	}
	if err := s.kafkaTopicJob(); err != nil {
		return err
	}
	for _, name := range []string{"kafka-ui", "jaeger", "otel-collector", "alloy", "postgres-exporter-sharetrip", "postgres-exporter-contract", "postgres-exporter-notification"} {
		if err := s.wait("deployment", name); err != nil {
			return err
		}
	}
	for _, app := range apps {
		if err := s.apply("k8s/" + app + "-config.yaml"); err != nil {
			return err
		}
		if err := s.apply("k8s/" + app + "-deployment.yaml"); err != nil {
			return err
		}
		// Local image tags are reused after every build. Replace running pods as well.
		if err := s.kube("-n", namespace, "rollout", "restart", "deployment/"+app); err != nil {
			return err
		}
		if err := s.wait("deployment", app); err != nil {
			return err
		}
	}
	if err := s.apply("k8s/infra/access.yaml"); err != nil {
		return err
	}
	if err := s.verify(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.local, "ready"), []byte(time.Now().Format(time.RFC3339)), 0600); err != nil {
		return err
	}
	fmt.Println("Ready: ShareTrip http://localhost:38080; Contract http://localhost:38082; Notification http://localhost:38081")
	fmt.Println("Keycloak http://localhost:38087; Grafana http://localhost:33000; Prometheus http://localhost:39090")
	fmt.Println("Kafka UI http://localhost:38085; Jaeger http://localhost:36686")
	return nil
}

func (s *stack) build() error {
	fmt.Println("Building applications...")
	for _, app := range apps {
		if err := s.docker("build", "-t", "sharetrip/"+app+":local", s.projects[app]); err != nil {
			return err
		}
	}
	archive := filepath.Join(s.local, "images.tar")
	if err := s.docker("image", "save", "--platform", "linux/amd64", "-o", archive, "sharetrip/sharetrip:local", "sharetrip/contract:local", "sharetrip/notification:local"); err != nil {
		return err
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	cmd := s.command("docker", "--context", "desktop-linux", "exec", "-i", "desktop-control-plane", "ctr", "-n", "k8s.io", "images", "import", "--local", "--platform", "linux/amd64", "--snapshotter", "overlayfs", "-")
	cmd.Stdin = file
	return execute(cmd)
}

func copyCommand(cmd *exec.Cmd, path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	cmd.Stdout, cmd.Stderr = file, os.Stderr
	return cmd.Run()
}

func (s *stack) checkCluster() error {
	data, err := s.capture("kubectl", "--context", "docker-desktop", "get", "namespace", namespace, "-o", "jsonpath={.metadata.uid}")
	if err != nil {
		return err
	}
	path := filepath.Join(s.local, "namespace-uid")
	previous, err := os.ReadFile(path)
	if err == nil && string(previous) != string(data) {
		return errors.New("namespace was recreated; inspect .local/k8s backups before initializing new storage")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
