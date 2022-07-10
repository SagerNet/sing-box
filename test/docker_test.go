package main

import (
	"context"
	"testing"
	"time"

	F "github.com/sagernet/sing/common/format"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

type DockerOptions struct {
	Image      string
	EntryPoint string
	Ports      []uint16
	Cmd        []string
	Env        []string
	Bind       []string
}

func startDockerContainer(t *testing.T, options DockerOptions) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer dockerClient.Close()

	var containerOptions container.Config
	containerOptions.Image = options.Image
	containerOptions.Entrypoint = []string{options.EntryPoint}
	containerOptions.Cmd = options.Cmd
	containerOptions.Env = options.Env
	containerOptions.ExposedPorts = make(nat.PortSet)

	var hostOptions container.HostConfig
	if !isDarwin {
		hostOptions.NetworkMode = "host"
	}
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

	dockerContainer, err := dockerClient.ContainerCreate(context.Background(), &containerOptions, &hostOptions, nil, nil, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanContainer(dockerContainer.ID)
	})
	require.NoError(t, dockerClient.ContainerStart(context.Background(), dockerContainer.ID, types.ContainerStartOptions{}))
	/*attach, err := dockerClient.ContainerAttach(context.Background(), dockerContainer.ID, types.ContainerAttachOptions{
		Logs: true, Stream: true, Stdout: true, Stderr: true,
	})
	require.NoError(t, err)
	go func() {
		attach.Reader.WriteTo(os.Stderr)
	}()*/
	time.Sleep(time.Second)
}

func cleanContainer(id string) error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer dockerClient.Close()
	return dockerClient.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
}
