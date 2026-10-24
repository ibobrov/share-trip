package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func (s *stack) job(name string, container map[string]any, volumes []any) error {
	if err := s.kube("-n", namespace, "delete", "job", name, "--ignore-not-found", "--wait=true"); err != nil {
		return err
	}
	job := resource("Job", name)
	job["apiVersion"] = "batch/v1"
	job["spec"] = map[string]any{
		"backoffLimit": 0, "activeDeadlineSeconds": 180,
		"template": map[string]any{"spec": map[string]any{
			"restartPolicy": "Never", "automountServiceAccountToken": false, "containers": []any{container}, "volumes": volumes,
		}},
	}
	if err := s.applyObject(job); err != nil {
		return err
	}
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		data, err := s.capture("kubectl", "--context", "docker-desktop", "-n", namespace, "get", "job", name, "-o", "json")
		if err != nil {
			return err
		}
		var job struct {
			Status struct{ Succeeded, Failed int }
		}
		if err := json.Unmarshal(data, &job); err != nil {
			return err
		}
		if job.Status.Succeeded > 0 {
			fmt.Println("Completed job", name)
			return nil
		}
		if job.Status.Failed > 0 {
			return fmt.Errorf("job %s failed; inspect kubectl logs -n sharetrip job/%s", name, name)
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("job %s timed out; inspect kubectl logs -n sharetrip job/%s", name, name)
}

func secretEnv(name, secret, key string) map[string]any {
	return map[string]any{"name": name, "valueFrom": map[string]any{"secretKeyRef": map[string]string{"name": secret, "key": key}}}
}

func (s *stack) exporterJob(app string) error {
	return s.job("exporter-init-"+app, map[string]any{
		"name": "init", "image": "postgres:16", "imagePullPolicy": "IfNotPresent",
		"command": []string{"sh", "/scripts/init.sh"},
		"env": []any{
			map[string]string{"name": "PGHOST", "value": "postgres-" + app},
			map[string]string{"name": "PGCONNECT_TIMEOUT", "value": "2"},
			secretEnv("PGUSER", "postgres-"+app+"-secret", "POSTGRES_USER"),
			secretEnv("PGPASSWORD", "postgres-"+app+"-secret", "POSTGRES_PASSWORD"),
			secretEnv("PGDATABASE", "postgres-"+app+"-secret", "POSTGRES_DB"),
			secretEnv("POSTGRES_EXPORTER_PASSWORD", "monitoring-secret", "POSTGRES_EXPORTER_PASSWORD"),
		},
		"volumeMounts": []any{map[string]any{"name": "sql", "mountPath": "/scripts", "readOnly": true}},
	}, []any{map[string]any{"name": "sql", "configMap": map[string]string{"name": "exporter-init"}}})
}

func (s *stack) kafkaTopicJob() error {
	return s.job("kafka-init", map[string]any{
		"name": "init", "image": "apache/kafka:4.1.2", "imagePullPolicy": "IfNotPresent",
		"command": []string{"/opt/kafka/bin/kafka-topics.sh", "--bootstrap-server", "kafka:9092", "--create", "--if-not-exists", "--topic", "trip.events", "--partitions", "3", "--replication-factor", "1"},
	}, nil)
}
