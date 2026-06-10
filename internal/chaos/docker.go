package chaos

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Docker is a thin wrapper around the `docker` CLI for the privileged mesh
// operations the chaos suite needs. Every method shells out; there is no
// daemon SDK dependency on purpose (keeps the suite trivially auditable and
// matches how compose/bootstrap.sh drives the stack).
type Docker struct {
	bin    string
	logger *slog.Logger
}

// NewDocker returns a Docker wrapper using the `docker` binary on PATH.
func NewDocker(logger *slog.Logger) *Docker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Docker{bin: "docker", logger: logger}
}

// run executes `docker <args...>` and returns trimmed combined output.
func (d *Docker) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, d.bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	d.logger.Debug("docker exec", "args", strings.Join(args, " "))
	err := cmd.Run()
	s := strings.TrimSpace(out.String())
	if err != nil {
		return s, fmt.Errorf("docker %s: %w (%s)", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// Available reports whether the docker CLI can talk to a daemon.
func (d *Docker) Available(ctx context.Context) error {
	if _, err := exec.LookPath(d.bin); err != nil {
		return fmt.Errorf("docker CLI not found on PATH: %w", err)
	}
	if _, err := d.run(ctx, "info", "--format", "{{.ServerVersion}}"); err != nil {
		return fmt.Errorf("docker daemon not reachable: %w", err)
	}
	return nil
}

// NetworkDisconnect detaches container from network (privileged partition).
func (d *Docker) NetworkDisconnect(ctx context.Context, network, container string) error {
	_, err := d.run(ctx, "network", "disconnect", network, container)
	return err
}

// NetworkConnect re-attaches container to network, re-establishing the given
// DNS aliases.
//
// This matters: `docker network disconnect` drops the network-scoped aliases
// (the compose service name and container name) that docker-compose assigns at
// `up` time, and a plain `docker network connect` does NOT restore them. A node
// reconnected without its aliases is reachable only by IP — so any peer that
// (re)starts afterward and resolves it by name (e.g. a Teranode whose legacy
// connect-peers list contains "svnode-3:18444") fails DNS resolution and, for
// the all-in-one daemon, aborts its entire startup. Callers therefore pass the
// container's compose aliases (service short name + container name) so the heal
// is faithful to the original `up` topology.
func (d *Docker) NetworkConnect(ctx context.Context, network, container string, aliases ...string) error {
	args := []string{"network", "connect"}
	for _, a := range aliases {
		if a != "" {
			args = append(args, "--alias", a)
		}
	}
	args = append(args, network, container)
	_, err := d.run(ctx, args...)
	return err
}

// NetworkContains reports whether container is currently attached to network.
func (d *Docker) NetworkContains(ctx context.Context, network, container string) (bool, error) {
	out, err := d.run(ctx, "network", "inspect", network,
		"--format", "{{range .Containers}}{{.Name}} {{end}}")
	if err != nil {
		return false, err
	}
	for _, name := range strings.Fields(out) {
		if name == container {
			return true, nil
		}
	}
	return false, nil
}

// Kill sends SIGKILL to a container (abrupt crash simulation).
func (d *Docker) Kill(ctx context.Context, container string) error {
	_, err := d.run(ctx, "kill", container)
	return err
}

// Start (re)starts a stopped container.
func (d *Docker) Start(ctx context.Context, container string) error {
	_, err := d.run(ctx, "start", container)
	return err
}

// Restart restarts a running container.
func (d *Docker) Restart(ctx context.Context, container string) error {
	_, err := d.run(ctx, "restart", container)
	return err
}

// Running reports whether a container's State.Running is true.
func (d *Docker) Running(ctx context.Context, container string) (bool, error) {
	out, err := d.run(ctx, "inspect", "-f", "{{.State.Running}}", container)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// Exec runs `docker exec <container> <args...>` and returns trimmed output.
func (d *Docker) Exec(ctx context.Context, container string, args ...string) (string, error) {
	full := append([]string{"exec", container}, args...)
	return d.run(ctx, full...)
}
