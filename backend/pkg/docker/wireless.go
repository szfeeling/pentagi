package docker

import (
	"context"
	"fmt"

	"pentagi/pkg/wireless"

	"github.com/moby/moby/client"
)

func WirelessRuntime(ctx context.Context, base DockerClient, binding wireless.NetworkContext) (DockerClient, error) {
	dc, ok := base.(*dockerClient)
	if !ok || dc.inside || dc.orchestratorDaemonID != binding.DaemonID {
		return nil, fmt.Errorf("wireless runtime requires the enrolled Linux Docker daemon and DOCKER_INSIDE=false")
	}
	inspected, err := dc.client.NetworkInspect(ctx, binding.NetworkID, client.NetworkInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect wireless network: %w", err)
	}
	n := inspected.Network
	if n.ID != binding.NetworkID || n.Name != binding.Network || n.Options["parent"] != binding.Interface || (n.Driver != "macvlan" && n.Driver != "ipvlan") {
		return nil, fmt.Errorf("wireless runtime network identity or interface does not match")
	}
	scoped := *dc
	scoped.network = binding.NetworkID
	return &scoped, nil
}

func CheckWirelessContainer(ctx context.Context, runtime DockerClient, id string, binding wireless.NetworkContext) error {
	dc, ok := runtime.(*dockerClient)
	if !ok {
		return fmt.Errorf("wireless runtime is unavailable")
	}
	inspected, err := dc.client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return err
	}
	if inspected.Container.NetworkSettings == nil {
		return fmt.Errorf("wireless container has no network")
	}
	for _, n := range inspected.Container.NetworkSettings.Networks {
		if n.NetworkID == binding.NetworkID && len(inspected.Container.NetworkSettings.Networks) == 1 {
			return nil
		}
	}
	return fmt.Errorf("container is outside the bound wireless runtime")
}
