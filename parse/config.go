package parse

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/engine-api/types"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/specs/specs-go"
)

const (
	// SpecVersion is the version of the opencontainers spec that will be created.
	SpecVersion = "0.5.0"

	// DefaultApparmorProfile is docker engine's default apparmor profile for containers.
	DefaultApparmorProfile = "docker-default"

	// DefaultCurrentWorkingDirectory defines the default directory the container
	// will enter in, if not already specified by the user.
	DefaultCurrentWorkingDirectory = "/"

	// DefaultTerminal is the default TERM for containers.
	DefaultTerminal = "xterm"

	// DefaultUserNSHostID is the default start mapped host id for userns.
	DefaultUserNSHostID = 886432
	// DefaultUserNSMapSize is the default size for the uid and gid mappings for userns.
	DefaultUserNSMapSize = 46578392
)

var (
	// DefaultTerminalEnv holds the minimum terminal env vars needed for an
	// interactive session.
	DefaultTerminalEnv = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	// DefaultMounts are the default mounts for a container.
	DefaultMounts = []specs.Mount{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
			Options:     nil,
		},
		{
			Destination: "/dev",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
		},
		{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"nosuid", "noexec", "nodev", "relatime"},
		},
	}

	// NetworkMounts are the mounts needed for default networking.
	NetworkMounts = []specs.Mount{
		{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      "/etc/hosts",
			Options:     []string{"rbind", "rprivate", "ro"},
		},
		{
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      "/etc/resolv.conf",
			Options:     []string{"rbind", "rprivate", "ro"},
		},
	}

	// Some variables for system container generation
	//terminal string
)

// Config takes ContainerJSON and converts it into the opencontainers spec.
func Config(c types.ContainerJSON, osType, architecture string, capabilities []string, idroot, idlen uint32, system bool, args, env []string) (config *specs.Spec, err error) {
	// for user namespaces use defaults unless another range specified
	if idroot == 0 {
		idroot = DefaultUserNSHostID
	}
	if idlen == 0 {
		idlen = DefaultUserNSMapSize
	}

	if !system {
		config = &specs.Spec{
			Version: SpecVersion,
			Platform: specs.Platform{
				OS:   osType,
				Arch: architecture,
			},
			Process: specs.Process{
				Terminal: c.Config.Tty,
				User:     specs.User{
				// TODO: user stuffs
				},
				Args: append([]string{c.Path}, c.Args...),
				Env:  c.Config.Env,
				Cwd:  c.Config.WorkingDir,
				// TODO: add parsing of Ulimits
				Rlimits: []specs.Rlimit{
					{
						Type: "RLIMIT_NOFILE",
						Hard: uint64(1024),
						Soft: uint64(1024),
					},
				},
				NoNewPrivileges: true,
				ApparmorProfile: c.AppArmorProfile,
			},
			Root: specs.Root{
				Path:     "rootfs",
				Readonly: c.HostConfig.ReadonlyRootfs,
			},
			Mounts: []specs.Mount{},
			Linux: specs.Linux{
				Namespaces: []specs.Namespace{
					{
						Type: "ipc",
					},
					{
						Type: "uts",
					},
					{
						Type: "mount",
					},
				},
				UIDMappings: []specs.IDMapping{
					{
						ContainerID: 0,
						HostID:      idroot,
						Size:        idlen,
					},
				},
				GIDMappings: []specs.IDMapping{
					{
						ContainerID: 0,
						HostID:      idroot,
						Size:        idlen,
					},
				},
				Resources: &specs.Resources{
					Devices: []specs.DeviceCgroup{
						{
							Allow:  false,
							Access: sPtr("rwm"),
						},
					},
					DisableOOMKiller: c.HostConfig.Resources.OomKillDisable,
					OOMScoreAdj:      &c.HostConfig.OomScoreAdj,
					Memory: &specs.Memory{
						Limit:       uint64ptr(c.HostConfig.Resources.Memory),
						Reservation: uint64ptr(c.HostConfig.Resources.MemoryReservation),
						Swap:        uint64ptr(c.HostConfig.Resources.MemorySwap),
						Swappiness:  uint64ptr(*c.HostConfig.Resources.MemorySwappiness),
						Kernel:      uint64ptr(c.HostConfig.Resources.KernelMemory),
					},
					CPU: &specs.CPU{
						Shares: uint64ptr(c.HostConfig.Resources.CPUShares),
						Quota:  uint64ptr(c.HostConfig.Resources.CPUQuota),
						Period: uint64ptr(c.HostConfig.Resources.CPUPeriod),
						Cpus:   &c.HostConfig.Resources.CpusetCpus,
						Mems:   &c.HostConfig.Resources.CpusetMems,
					},
					Pids: &specs.Pids{
						Limit: &c.HostConfig.Resources.PidsLimit,
					},
					BlockIO: &specs.BlockIO{
						Weight: &c.HostConfig.Resources.BlkioWeight,
						// TODO: add parsing for Throttle/Weight Devices
					},
				},
				RootfsPropagation: "",
			},
		}
	} else {
		config = &specs.Spec{
			Version: SpecVersion,
			Platform: specs.Platform{
				OS:   osType,
				Arch: architecture,
			},
			Process: specs.Process{
				Terminal: false,
				User:     specs.User{
					UID: 0,
					GID: 0,
				},
				Args: append([]string{c.Path}, c.Args...),
				Env:  c.Config.Env,
				Cwd:  c.Config.WorkingDir,
				Rlimits: []specs.Rlimit{
					{
						Type: "RLIMIT_NOFILE",
						Hard: uint64(1024),
						Soft: uint64(1024),
					},
				},
			},
			Root: specs.Root{
				Path:     "rootfs",
				Readonly: true,
			},
			Mounts: []specs.Mount{},
			Linux: specs.Linux{
				Namespaces: []specs.Namespace{
					{
						Type: "ipc",
					},
					{
						Type: "uts",
					},
					{
						Type: "mount",
					},
				},
				Resources: &specs.Resources{
					Devices: []specs.DeviceCgroup{
						{
							Allow:  false,
							Access: sPtr("rwm"),
						},
					},
				},
			},
		}
	}


	if len(args) > 0 {
		config.Process.Args = args
	}

	if len(env) > 0 {
		config.Process.Env = append(config.Process.Env, env...)
	}

	// make sure the current working directory is not blank
	if config.Process.Cwd == "" {
		config.Process.Cwd = DefaultCurrentWorkingDirectory
	}

	// get the user
	if c.Config.User != "" && !system {
		u, err := user.LookupUser(c.Config.User)
		if err != nil {
			config.Process.User = specs.User{
				UID: uint32(u.Uid),
				GID: uint32(u.Gid),
			}
		} else {
			//return nil, fmt.Errorf("Looking up user (%s) failed: %v", c.Config.User, err)
			logrus.Warnf("Looking up user (%s) failed: %v", c.Config.User, err)
		}
	}
	// add the additional groups
	for _, group := range c.HostConfig.GroupAdd {
		g, err := user.LookupGroup(group)
		if err != nil {
			return nil, fmt.Errorf("Looking up group (%s) failed: %v", group, err)
		}
		config.Process.User.AdditionalGids = append(config.Process.User.AdditionalGids, uint32(g.Gid))
	}

	// get the hostname, if the hostname is the name as the first 12 characters of the id,
	// then set the hostname as the container name
	if c.ID[:12] == c.Config.Hostname && !system {
		config.Hostname = strings.TrimPrefix(c.Name, "/")
	}

	// set privileged
	if c.HostConfig.Privileged {
		// allow all caps
		capabilities = execdriver.GetAllCapabilities()
	}

	// get the capabilities
	config.Process.Capabilities, err = execdriver.TweakCapabilities(capabilities, c.HostConfig.CapAdd, c.HostConfig.CapDrop)
	if err != nil {
		return nil, fmt.Errorf("setting capabilities failed: %v", err)
	}

	// add CAP_ prefix
	// TODO: this is awful
	for i, cap := range config.Process.Capabilities {
		if !strings.HasPrefix(cap, "CAP_") {
			config.Process.Capabilities[i] = fmt.Sprintf("CAP_%s", cap)
		}
	}

	// if we have a container that needs a terminal but no env vars, then set
	// default env vars for the terminal to function
	if config.Process.Terminal && len(config.Process.Env) <= 0 {
		config.Process.Env = DefaultTerminalEnv
	}
	if config.Process.Terminal {
		// make sure we have TERM set
		var termSet bool
		for _, env := range config.Process.Env {
			if strings.HasPrefix(env, "TERM=") {
				termSet = true
				break
			}
		}
		if !termSet {
			// set the term variable
			config.Process.Env = append(config.Process.Env, fmt.Sprintf("TERM=%s", DefaultTerminal))
		}
	}

	// check namespaces
	if !c.HostConfig.NetworkMode.IsHost() && !system {
		config.Linux.Namespaces = append(config.Linux.Namespaces, specs.Namespace{
			Type: "network",
		})
	}
	if !c.HostConfig.PidMode.IsHost() {
		config.Linux.Namespaces = append(config.Linux.Namespaces, specs.Namespace{
			Type: "pid",
		})
	}
	if c.HostConfig.UsernsMode.Valid() && !c.HostConfig.NetworkMode.IsHost() && !c.HostConfig.PidMode.IsHost() && !c.HostConfig.Privileged && !system {
		config.Linux.Namespaces = append(config.Linux.Namespaces, specs.Namespace{
			Type: "user",
		})
	} else {
		// reset uid and gid mappings
		config.Linux.UIDMappings = []specs.IDMapping{}
		config.Linux.GIDMappings = []specs.IDMapping{}
	}

	// get mounts
	mounts := map[string]bool{}
	for _, mount := range c.Mounts {
		mounts[mount.Destination] = true
		var opt []string
		if mount.RW {
			opt = append(opt, "rw")
		}
		if mount.Mode != "" {
			opt = append(opt, mount.Mode)
		}
		opt = append(opt, []string{"rbind", "rprivate"}...)

		config.Mounts = append(config.Mounts, specs.Mount{
			Destination: mount.Destination,
			Type:        "bind",
			Source:      mount.Source,
			Options:     opt,
		})
	}

	// add /etc/hosts and /etc/resolv.conf if we should have networking
	if c.HostConfig.NetworkMode != "none" && c.HostConfig.NetworkMode != "host" {
		DefaultMounts = append(DefaultMounts, NetworkMounts...)
	}

	// if we aren't doing something crazy like mounting a default mount ourselves,
	// the we can mount it the default way
	for _, mount := range DefaultMounts {
		if _, ok := mounts[mount.Destination]; !ok {
			config.Mounts = append(config.Mounts, mount)
		}
	}

	// fix default mounts for cgroups and devpts without user namespaces
	// see: https://github.com/opencontainers/runc/issues/225#issuecomment-136519577
	if len(config.Linux.UIDMappings) == 0 {
		for k, mount := range config.Mounts {
			switch mount.Destination {
			case "/sys/fs/cgroup":
				config.Mounts[k].Options = append(config.Mounts[k].Options, "ro")
			case "/dev/pts":
				config.Mounts[k].Options = append(config.Mounts[k].Options, "gid=5")
			}
		}
	}

	// parse additional groups and add them to gid mappings
	if err := parseMappings(config, c.HostConfig); err != nil {
		return nil, err
	}

	if !system {
		// parse devices
		if err := parseDevices(config, c.HostConfig); err != nil {
			return nil, err
		}

		// parse security opt
		if err := parseSecurityOpt(config, c.HostConfig); err != nil {
			return nil, err
		}
	}

	// set privileged
	if c.HostConfig.Privileged {
		if !c.HostConfig.ReadonlyRootfs {
			// clear readonly for cgroup
			//	config.Mounts["cgroup"] = DefaultMountpoints["cgroup"]
		}
	}

	return config, nil
}
