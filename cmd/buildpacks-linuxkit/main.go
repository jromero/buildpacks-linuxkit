package main

import (
	"context"
	"log"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

const (
	ns               = "buildpacks"
	builderImage     = "index.docker.io/cnbs/sample-builder:alpine"
	builderContainer = "builder"
)

func main() {
	address := "dockerless-containerd-efi-state/guest.00000946"
	log.Println("Connecting to:", address)
	client, err := containerd.New(address)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	log.Printf("Using namespace '%s'...", ns)
	ctx := namespaces.WithNamespace(context.Background(), ns)

	log.Println("Looking up containers...")
	containers, err := client.Containers(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if len(containers) == 0 {
		log.Println("No containers found in namespace.")
	} else {
		log.Println("Containers:")
		for _, container := range containers {
			log.Println(container.ID())
		}
	}

	log.Println("Pulling image: ", builderImage)
	image, err := client.Pull(ctx, builderImage, containerd.WithPullUnpack)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Creating container: ", builderContainer)
	container, err := client.NewContainer(
		ctx,
		builderContainer,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(builderContainer+"-snapshot", image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)
}
