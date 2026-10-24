package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type volume struct{ name, container, source, target, owner string }

// Preserve complete stores, including database roles, Kafka offsets and Keycloak users.
var volumes = []volume{
	{"postgres-sharetrip", "pg", "/var/lib/postgresql/data", "pgdata", "999:999"},
	{"postgres-contract", "pg-contract", "/var/lib/postgresql/data", "pgdata", "999:999"},
	{"postgres-notification", "pg-notification", "/var/lib/postgresql/data", "pgdata", "999:999"},
	{"kafka", "sharetrip-kafka", "/var/lib/kafka/data", ".", "1000:1000"},
	{"keycloak", "sharetrip-keycloak", "/opt/keycloak/data", ".", "1000:0"},
	{"grafana", "grafana", "/var/lib/grafana", ".", "472:0"},
	{"loki", "loki", "/loki", ".", "10001:10001"},
	{"prometheus", "prometheus", "/prometheus", ".", "65534:65534"},
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func (s *stack) migrateCompose() error {
	if exists(filepath.Join(s.local, "storage-ready")) {
		return nil
	}
	active, err := s.capture("kubectl", "--context", "docker-desktop", "-n", namespace, "get", "statefulset", "-o", "name")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(active)) != "" {
		return errors.New("storage initialization marker is missing but infrastructure already exists; inspect .local/k8s before restoring any data")
	}
	fmt.Println("Preparing persistent storage; existing Compose data will be kept in .local/k8s/backup.")
	backup := filepath.Join(s.local, "backup")
	if err := os.MkdirAll(backup, 0700); err != nil {
		return err
	}
	if !exists(filepath.Join(backup, "complete")) {
		present := []volume{}
		for _, v := range volumes {
			cmd := s.command("docker", "--context", "desktop-linux", "container", "inspect", v.container)
			if err := cmd.Run(); err == nil {
				present = append(present, v)
			}
		}
		if len(present) != 0 && len(present) != len(volumes) {
			return errors.New("incomplete Compose stack: restore missing source containers or migrate data manually")
		}
		// Freeze writers before taking any backup. Never delete their containers or volumes.
		if err := s.kube("-n", namespace, "scale", "deployment", "--all", "--replicas=0"); err != nil {
			return err
		}
		if err := s.kube("-n", namespace, "wait", "--for=delete", "pod", "-l", "app in (sharetrip,contract,notification)", "--timeout=90s"); err != nil {
			return err
		}
		for _, v := range present {
			if err := s.docker("stop", v.container); err != nil {
				return err
			}
		}
		for _, v := range present {
			fmt.Println("Backing up", v.name)
			cmd := s.command("docker", "--context", "desktop-linux", "cp", v.container+":"+v.source+"/.", "-")
			if err := copyCommand(cmd, filepath.Join(backup, v.name+".tar")); err != nil {
				return err
			}
		}
		// Preserve the previous application configuration for manual rollback.
		for _, kind := range []string{"configmap", "secret"} {
			if err := copyCommand(s.command("kubectl", "--context", "docker-desktop", "-n", namespace, "get", kind, "-o", "json"), filepath.Join(backup, kind+".json")); err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(backup, "complete"), []byte("cold backups complete\n"), 0600); err != nil {
			return err
		}
	}
	if err := s.apply("k8s/infra/storage.yaml"); err != nil {
		return err
	}
	// One temporary pod binds all local PVCs to the same Docker Desktop node.
	if err := s.kube("-n", namespace, "delete", "pod", "storage-seed", "--ignore-not-found", "--wait=true"); err != nil {
		return err
	}
	pod := resource("Pod", "storage-seed")
	mounts, claims := []any{}, []any{}
	for _, v := range volumes {
		mounts = append(mounts, map[string]any{"name": v.name, "mountPath": "/data/" + v.name})
		claims = append(claims, map[string]any{"name": v.name, "persistentVolumeClaim": map[string]string{"claimName": v.name}})
	}
	pod["spec"] = map[string]any{"restartPolicy": "Never", "automountServiceAccountToken": false, "containers": []any{map[string]any{
		"name": "seed", "image": "postgres:16", "imagePullPolicy": "IfNotPresent", "command": []string{"sleep", "3600"}, "volumeMounts": mounts,
	}}, "volumes": claims}
	if err := s.applyObject(pod); err != nil {
		return err
	}
	if err := s.kube("-n", namespace, "wait", "--for=condition=Ready", "pod/storage-seed", "--timeout=3m"); err != nil {
		return err
	}
	for _, v := range volumes {
		target := "/data/" + v.name + "/" + v.target
		if err := s.kube("-n", namespace, "exec", "storage-seed", "--", "mkdir", "-p", target); err != nil {
			return err
		}
		archive := filepath.Join(backup, v.name+".tar")
		if exists(archive) {
			fmt.Println("Restoring", v.name)
			file, err := os.Open(archive)
			if err != nil {
				return err
			}
			cmd := s.command("kubectl", "--context", "docker-desktop", "-n", namespace, "exec", "-i", "storage-seed", "--", "tar", "-xpf", "-", "-C", target)
			cmd.Stdin = file
			err = execute(cmd)
			_ = file.Close()
			if err != nil {
				return err
			}
		}
		if err := s.kube("-n", namespace, "exec", "storage-seed", "--", "chown", "-R", v.owner, target); err != nil {
			return err
		}
		if v.target == "pgdata" {
			if err := s.kube("-n", namespace, "exec", "storage-seed", "--", "chmod", "700", target); err != nil {
				return err
			}
		}
	}
	if err := s.kube("-n", namespace, "delete", "pod", "storage-seed", "--wait=true"); err != nil {
		return err
	}
	// Do not ever copy old Compose data again after the first successful restore.
	if err := os.WriteFile(filepath.Join(s.local, "storage-ready"), []byte("PVCs initialized\n"), 0600); err != nil {
		return err
	}
	for _, name := range []string{"otel_collector", "alloy", "pg_exporter", "sharetrip-kafka-ui", "jaeger"} {
		cmd := s.command("docker", "--context", "desktop-linux", "container", "inspect", name)
		if err := cmd.Run(); err == nil {
			if err := s.docker("stop", name); err != nil {
				return err
			}
		}
	}
	return nil
}
