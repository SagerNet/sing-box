package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagernet/sing/common/debug"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/rw"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

type DockerOptions struct {
	Image      string
	EntryPoint string
	Ports      []uint16
	Cmd        []string
	Env        []string
	Bind       map[string]string
	Stdin      []byte
	Cap        []string
}

func startDockerContainer(t *testing.T, options DockerOptions) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer dockerClient.Close()

	writeStdin := len(options.Stdin) > 0

	var containerOptions container.Config

	if writeStdin {
		containerOptions.OpenStdin = true
		containerOptions.StdinOnce = true
	}

	containerOptions.Image = options.Image
	if options.EntryPoint != "" {
		containerOptions.Entrypoint = []string{options.EntryPoint}
	}
	containerOptions.Cmd = options.Cmd
	containerOptions.Env = options.Env
	containerOptions.ExposedPorts = make(nat.PortSet)

	var hostOptions container.HostConfig
	hostOptions.NetworkMode = "host"
	hostOptions.CapAdd = options.Cap
	hostOptions.PortBindings = make(nat.PortMap)

	for _, port := range options.Ports {
		containerOptions.ExposedPorts[nat.Port(F.ToString(port, "/tcp"))] = struct{}{}
		containerOptions.ExposedPorts[nat.Port(F.ToString(port, "/udp"))] = struct{}{}
		hostOptions.PortBindings[nat.Port(F.ToString(port, "/tcp"))] = []nat.PortBinding{
			{HostPort: F.ToString(port), HostIP: "0.0.0.0"},
		}
		hostOptions.PortBindings[nat.Port(F.ToString(port, "/udp"))] = []nat.PortBinding{
			{HostPort: F.ToString(port), HostIP: "0.0.0.0"},
		}
	}

	if len(options.Bind) > 0 {
		hostOptions.Binds = []string{}
		for path, internalPath := range options.Bind {
			if !rw.FileExists(path) {
				path = filepath.Join("config", path)
			}
			path, _ = filepath.Abs(path)
			hostOptions.Binds = append(hostOptions.Binds, path+":"+internalPath)
		}
	}

	dockerContainer, err := dockerClient.ContainerCreate(context.Background(), &containerOptions, &hostOptions, nil, nil, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanContainer(dockerContainer.ID)
	})

	require.NoError(t, dockerClient.ContainerStart(context.Background(), dockerContainer.ID, container.StartOptions{}))

	if writeStdin {
		stdinAttach, err := dockerClient.ContainerAttach(context.Background(), dockerContainer.ID, container.AttachOptions{
			Stdin:  writeStdin,
			Stream: true,
		})
		require.NoError(t, err)
		_, err = stdinAttach.Conn.Write(options.Stdin)
		require.NoError(t, err)
		stdinAttach.Close()
	}
	if debug.Enabled {
		attach, err := dockerClient.ContainerAttach(context.Background(), dockerContainer.ID, container.AttachOptions{
			Stdout: true,
			Stderr: true,
			Logs:   true,
			Stream: true,
		})
		require.NoError(t, err)
		go func() {
			stdcopy.StdCopy(os.Stderr, os.Stderr, attach.Reader)
		}()
	}
	time.Sleep(time.Second)
}

func cleanContainer(id string) error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer dockerClient.Close()
	return dockerClient.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
}
