package tutils

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/osbuild/image-builder-crc/internal/db"
)

type PSQLContainer struct {
	name string
	id   string
	port int
}

const (
	image string = "quay.io/osbuild/postgres:13-alpine"
	user  string = "postgres"
)

func containerRuntime() (string, error) {
	out, err := exec.Command("which", "podman").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	out, err = exec.Command("which", "docker").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	return "", fmt.Errorf("no container runtime found (looked for podman or docker)")
}

func NewPSQLContainer() (*PSQLContainer, error) {
	rt, err := containerRuntime()
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("image_builder_test_%d", time.Now().Unix())
	/* #nosec G404 */
	port := 65535 - rand.Intn(32000)
	/* #nosec G204 */
	out, err := exec.Command(
		rt,
		"run",
		"--mount=type=tmpfs,destination=/var/lib/postgresql/data",
		"--mount=type=tmpfs,destination=/dev/shm",
		"--detach",
		"--rm",
		"--name", name,
		"--env", fmt.Sprintf("POSTGRES_USER=%s", user),
		"--env", "POSTGRES_HOST_AUTH_METHOD=trust",
		"-p", fmt.Sprintf("127.0.0.1:%d:5432", port),
		image,
	).Output()
	if err != nil {
		fmt.Println(out, err)
		return nil, err
	}

	p := &PSQLContainer{
		name: name,
		id:   strings.TrimSpace(string(out)),
		port: port,
	}

	tries := 0
	for tries < 10 {
		_, err := p.execCommand("exec", p.name, "pg_isready")
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}
		return p, nil
	}
	return p, fmt.Errorf("container not ready: %v", err)
}

func (p *PSQLContainer) execCommand(args ...string) (string, error) {
	rt, err := containerRuntime()
	if err != nil {
		return "", err
	}
	/* #nosec G204 */
	out, err := exec.Command(rt, args...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("command %s %s error: %w, output: %s", rt, args, err, out)
	}
	return strings.TrimSpace(string(out)), err
}

func (p *PSQLContainer) execQuery(dbase, cmd string) (string, error) {
	args := []string{
		"exec", p.name, "psql", "-U", user,
	}
	if dbase != "" {
		args = append(args, "-d", dbase)
	}
	args = append(args, "-c", fmt.Sprintf("%s;", cmd))
	return p.execCommand(args...)
}

func (p *PSQLContainer) Stop() error {
	_, err := p.execCommand("kill", p.name)
	return err
}

type TernMigrateOptions struct {
	MigrationsDir string
	Hostname      string
	DBName        string
	DBPort        string
	DBUser        string
	DBPassword    string
	SSLMode       string
}

func callTernMigrate(ctx context.Context, opt TernMigrateOptions) ([]byte, error) {
	args := []string{
		"migrate",
	}

	addArg := func(flag, value string) {
		if value != "" {
			args = append(args, flag, value)
		}
	}

	addArg("--migrations", opt.MigrationsDir)
	addArg("--host", opt.Hostname)
	addArg("--database", opt.DBName)
	addArg("--port", opt.DBPort)
	addArg("--user", opt.DBUser)
	addArg("--password", opt.DBPassword)
	addArg("--sslmode", opt.SSLMode)

	/* #nosec G204 */
	cmd := exec.CommandContext(ctx, "tern", args...)

	return cmd.CombinedOutput()
}

func (p *PSQLContainer) NewDB(ctx context.Context) (db.DB, error) {
	dbName := fmt.Sprintf("test%s", strings.Replace(uuid.New().String(), "-", "", -1))
	_, err := p.execQuery("", fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		return nil, err
	}

	out, err := callTernMigrate(
		ctx,
		TernMigrateOptions{
			MigrationsDir: "../db/migrations-tern/",
			Hostname:      "localhost",
			DBName:        dbName,
			DBPort:        fmt.Sprintf("%d", p.port),
			DBUser:        user,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("tern command error: %w, output: %s", err, out)
	}
	return db.InitDBConnectionPool(ctx, fmt.Sprintf("postgres://postgres@localhost:%d/%s", p.port, dbName))
}
