package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatabaseURLOverridesLegacyCredentials(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, ".env"), []byte("DATABASE_DSN=postgres://owner:p%40ss@localhost:6543/app\nPOSTGRES_USER=old\nPOSTGRES_PASSWORD=old\nPOSTGRES_DB=old\n"), 0600))
	s := stack{projects: map[string]string{"sharetrip": project}}
	env, err := s.environment("sharetrip")
	require.NoError(t, err)
	require.Equal(t, "owner", env["POSTGRES_USER"])
	require.Equal(t, "p@ss", env["POSTGRES_PASSWORD"])
	require.Equal(t, "app", env["POSTGRES_DB"])
}

func TestInvalidDatabaseURLDoesNotExposeCredentials(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, ".env"), []byte("DATABASE_DSN=postgres://owner:private%zz@localhost/app\n"), 0600))
	s := stack{projects: map[string]string{"sharetrip": project}}
	_, err := s.environment("sharetrip")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private")
}
