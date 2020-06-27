# Docker-less `pack` on macOS

## Goal

Eliminate the dependency on [Docker Desktop for Mac](https://hub.docker.com/editions/community/docker-ce-desktop-mac) on macOS.

In an ideal world, you would download `pack` and start using it with no additional dependencies.

### Log

#### 1. Replace Docker Desktop with LinuxKit and DinD

##### Goal

Similar to how Docker for Mac works, the idea would be to "embed" docker inside of a managed VM. This would retain the functionality but remove the application from needed to be installed.

```text
                     +-----------------------------------------------------+
                     |                                                     |
                     |  qemu / HyperKit  (swappable)                       |
                     |                                                     |
+-----------+        |         +---------------------------------------+   |
|           |        |         |                                       |   |
|   client  |        |         |  LinuxKit                             |   |
|           |        |         |                                       |   |
+-----------+        |         |      +----------------------------+   |   |
                     |         |      |                            |   |   |
      ^              |         |      |  containerd                |   |   |
      |              |         |      |                            |   |   |
      |              |         |      |   +--------------------+   |   |   |
      |              |         |      |   |                    |   |   |   |
      |              |         |      |   |  Docker-in-Docker  |   |   |   |
      +--+ socket +---------------------> |                    |   |   |   |
                     |         |      |   +--------------------+   |   |   |
                     |         |      |                            |   |   |
                     |         |      +----------------------------+   |   |
                     |         |                                       |   |
                     |         +---------------------------------------+   |
                     |                                                     |
                     +-----------------------------------------------------+
```

###### Log

First I wanted to remove Docker Desktop for Mac by using LinuxKit directly. As I found, the easiest step was to essentially using DinD inside LinuxKit so that the socket API continues to work as expected.  

```bash
brew tap linuxkit/linuxkit
brew install linuxkit
```

From: https://www.qemu.org/download/#macos
```bash
brew install qemu
```
NOTE:

From: https://github.com/linuxkit/linuxkit/blob/master/examples/docker-for-mac.md
```bash
linuxkit build -format iso-efi docker-for-mac.yml
linuxkit run hyperkit -networking=vpnkit -vsock-ports=2376 -disk size=4096M -data-file ./metadata.json -iso -uefi docker-for-mac-efi

# NOTE: another terminal
docker -H unix://docker-for-mac-efi-state/guest.00000948 ps
```

```bash
# NOTE: docker is not otherwise running locally
$ pack build my-app -B cnbs/sample-builder:alpine -p ~/dev/buildpacks/samples/apps/bash-script/
ERROR: failed to fetch builder image 'index.docker.io/cnbs/sample-builder:alpine': Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?

$ DOCKER_HOST=unix://docker-for-mac-efi-state/guest.00000948 pack build my-app -B cnbs/sample-builder:alpine -p ~/dev/buildpacks/samples/apps/bash-script/
alpine: Pulling from cnbs/sample-builder
21c83c524219: Pull complete
...
===> DETECTING
ERROR: container start: Error response from daemon: OCI runtime create failed: container_linux.go:349: starting container process caused "process_linux.go:449: container init caused \"process_linux.go:432: running prestart hook 0 caused \\\"fork/exec /proc/7/exe: no such file or directory\\\"\"": unknown
```

Found https://github.com/linuxkit/linuxkit/issues/3339

##### Conclusion

Without being able to start a container it is hard to tell how feasible this solution would be. It's still relatively heavy but otherwise the integration would be relatively light due to using exisiting integration points (docker client).

Next steps, this error seems like something that could be further investigated or resolved automatically in the upstream later in time. 

#### 2. Dockerless - LinuxKit and Containerd

##### Goal

Attempt to slim down the dependencies by communicating to containerd directly instead of going through the docker daemon.

```text
+-----------+        +------------------------------------------+
|           |        |                                          |
|   client  |        |  qemu / HyperKit  (swappable)            |
|           |        |                                          |
+-----------+        |                                          |
                     |         +----------------------------+   |
      ^              |         |                            |   |
      |              |         |  LinuxKit                  |   |
      |              |         |                            |   |
      |              |         |      +------------------+  |   |
      |              |         |      |                  |  |   |
      +--- socket +-----------------> |    containerd    |  |   |
                     |         |      |                  |  |   |
                     |         |      +------------------+  |   |
                     |         |                            |   |
                     |         |                            |   |
                     |         +----------------------------+   |
                     |                                          |
                     +------------------------------------------+
```

##### Log

```bash
linuxkit build -format iso-efi dockerless-containerd.yml
linuxkit run hyperkit -networking=vpnkit -vsock-ports=2374 -disk size=4096M -iso -uefi dockerless-containerd-efi
```

```bash
$ go run cmd/buildpacks-linuxkit/main.go

2020/06/27 13:33:47 Connecting to: dockerless-containerd-efi-state/guest.00000946
2020/06/27 13:33:47 Using namespace 'buildpacks'...
2020/06/27 13:33:47 Looking up containers...
2020/06/27 13:33:47 No containers found in namespace.
2020/06/27 13:33:47 Pulling image:  index.docker.io/cnbs/sample-builder:alpine
2020/06/27 13:33:54 Creating container:  builder
2020/06/27 13:33:55 failed to unmount /var/folders/nx/x67fz2nj5hv_w43gn5h019hh0000gn/T/containerd-mount434101365: not implemented under unix: failed to mount /var/folders/nx/x67fz2nj5hv_w43gn5h019hh0000gn/T/containerd-mount434101365: not implemented under unix
exit status 1

```

FOUND: https://github.com/containerd/containerd/issues/3910#issuecomment-568215764

> the Go client is a "fat client" in that, unlike the Docker client<-->server experience, the client does work on its own with an expectation of "seeing" the same content as the server. It is not simply a "dumb" client that just needs the containerd API socket. Therefore, similar to how LinuxKit set up containerd, there are a set of mountpoints you need to share between the client and server if you aren't going to run them on the host together (e.g. run the server containerized)

###### Conclusion

It appears that there is a strong limitation on how the `containerd` go library can be used via socket. It does A LOT more than just communication. It attempts to mutate the local file system as if it was **co-located**.

Next steps:

1. I wonder if there is a "dumb" client where a lot of the heavy lifting is done inside of the VM and it's basic requests via socket.
2. A custom service inside the VM can be built to translate requests and operate inside the VM where it would indeed be co-located.

### Resources
- LinuxKit qemo: https://github.com/linuxkit/linuxkit/blob/master/docs/platform-qemu.md
- LinuxKit Deep Dive: https://www.youtube.com/watch?v=pW_Ptz8R7Rg
- LinuxKit Under the Hood: https://www.youtube.com/watch?v=fIRaPGxhsH0
- `vsudd` port socket forwarding: https://github.com/linuxkit/linuxkit/blob/master/docs/platform-hyperkit.md#vsudd-unix-domain-socket-forwarding
- `containerd` go lib: https://containerd.io/docs/getting-started/