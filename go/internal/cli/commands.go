package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"gpunow/internal/cluster"
	"gpunow/internal/parse"
	"gpunow/internal/ssh"
	"gpunow/internal/target"
	"gpunow/internal/version"
	"gpunow/internal/vm"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:    "gpunow",
		Usage:   "Manage GPU VMs and clusters on GCP",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "profile",
				Aliases: []string{"p"},
				Value:   "default",
				Usage:   "Profile name",
				EnvVars: []string{"GPUNOW_PROFILE"},
			},
			&cli.StringFlag{
				Name:  "log-level",
				Value: "warn",
				Usage: "Log level (debug, info, warn, error)",
			},
		},
		Commands: []*cli.Command{
			installCommand(),
			vmCommand(),
			clusterCommand(),
			sshCommand(),
			scpCommand(),
			stateCommand(),
			versionCommand(),
		},
	}
	app.ExitErrHandler = func(c *cli.Context, err error) {
		if err == nil {
			return
		}
		fmt.Fprintln(c.App.ErrWriter, err)
	}
	return app
}

func vmCommand() *cli.Command {
	return &cli.Command{
		Name:  "vm",
		Usage: "Manage single VM instances",
		Subcommands: []*cli.Command{
			{
				Name:      "start",
				Usage:     "Start a VM (optionally by name or cluster/index)",
				ArgsUsage: "[name or cluster/index]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "ports", Usage: "Comma-separated ports to allow"},
					&cli.IntFlag{Name: "max-hours", Usage: "Max run duration in hours"},
				},
				Action: vmStart,
			},
			{
				Name:      "stop",
				Usage:     "Stop or delete a VM",
				ArgsUsage: "[name or cluster/index]",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "delete", Usage: "Delete the instance"},
					&cli.BoolFlag{Name: "keep-disks", Usage: "Preserve disks when deleting"},
				},
				Action: vmStop,
			},
			{
				Name:      "status",
				Aliases:   []string{"show"},
				Usage:     "Show VM status",
				ArgsUsage: "[name or cluster/index]",
				Action:    vmStatus,
			},
			{
				Name:      "update",
				Usage:     "Update VM settings",
				ArgsUsage: "[name or cluster/index]",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "max-hours", Usage: "Max run duration in hours"},
				},
				Action: vmUpdate,
			},
		},
	}
}

func clusterCommand() *cli.Command {
	return &cli.Command{
		Name:  "cluster",
		Usage: "Manage GPU clusters",
		Subcommands: []*cli.Command{
			{
				Name:      "start",
				Usage:     "Start a cluster",
				ArgsUsage: "<cluster>",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "num-instances", Aliases: []string{"n"}, Usage: "Number of instances"},
				},
				Action: clusterStart,
			},
			{
				Name:      "stop",
				Usage:     "Stop or delete a cluster",
				ArgsUsage: "<cluster>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "delete", Usage: "Delete instances"},
					&cli.BoolFlag{Name: "keep-disks", Usage: "Preserve disks when deleting"},
				},
				Action: clusterStop,
			},
			{
				Name:      "status",
				Aliases:   []string{"show"},
				Usage:     "Show cluster status",
				ArgsUsage: "<cluster>",
				Action:    clusterStatus,
			},
			{
				Name:      "update",
				Usage:     "Update cluster settings",
				ArgsUsage: "<cluster>",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "max-hours", Usage: "Max run duration in hours"},
				},
				Action: clusterUpdate,
			},
		},
	}
}

func sshCommand() *cli.Command {
	return &cli.Command{
		Name:      "ssh",
		Usage:     "SSH into a VM or cluster node",
		ArgsUsage: "[target]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "user", Aliases: []string{"u"}, Usage: "SSH username"},
		},
		Action: sshAction,
	}
}

func scpCommand() *cli.Command {
	return &cli.Command{
		Name:      "scp",
		Usage:     "SCP files to/from a cluster VM",
		ArgsUsage: "<src> <dst>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "user", Aliases: []string{"u"}, Usage: "SSH username"},
		},
		Action: scpAction,
	}
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print version information",
		Action: func(c *cli.Context) error {
			fmt.Fprintln(c.App.Writer, version.String())
			return nil
		},
	}
}

func vmStart(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	targetRef, err := resolveVMTarget(c, state)
	if err != nil {
		return err
	}
	name := targetRef.Name
	announce(state)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	ports := []int{}
	if c.IsSet("ports") {
		ports, err = parse.PortsCSV(c.String("ports"))
		if err != nil {
			return err
		}
	}

	maxHours := state.Config.Instance.MaxRunHours
	maxHoursSet := false
	if c.IsSet("max-hours") {
		maxHours = c.Int("max-hours")
		maxHoursSet = true
		if maxHours <= 0 {
			return fmt.Errorf("--max-hours must be a positive integer")
		}
	}

	service := vm.NewService(compute, state.Config, state.UI, state.Logger)
	return service.Start(c.Context, name, vm.StartOptions{
		Ports:         ports,
		MaxRunHours:   maxHours,
		MaxHoursSet:   maxHoursSet,
		AllowCreate:   !targetRef.IsCluster,
		SkipFirewall:  targetRef.IsCluster,
		SkipTagUpdate: targetRef.IsCluster,
	})
}

func vmStop(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	targetRef, err := resolveVMTarget(c, state)
	if err != nil {
		return err
	}
	name := targetRef.Name
	announce(state)

	if !c.Bool("delete") && c.Bool("keep-disks") {
		return fmt.Errorf("--keep-disks requires --delete")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := vm.NewService(compute, state.Config, state.UI, state.Logger)
	return service.Stop(c.Context, name, vm.StopOptions{
		Delete:    c.Bool("delete"),
		KeepDisks: c.Bool("keep-disks"),
	})
}

func vmStatus(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	targetRef, err := resolveVMTarget(c, state)
	if err != nil {
		return err
	}
	name := targetRef.Name
	announce(state)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := vm.NewService(compute, state.Config, state.UI, state.Logger)
	return service.Show(c.Context, name)
}

func vmUpdate(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	targetRef, err := resolveVMTarget(c, state)
	if err != nil {
		return err
	}
	name := targetRef.Name
	announce(state)

	if !c.IsSet("max-hours") {
		return fmt.Errorf("--max-hours is required for update")
	}
	if c.Int("max-hours") <= 0 {
		return fmt.Errorf("--max-hours must be a positive integer")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := vm.NewService(compute, state.Config, state.UI, state.Logger)
	return service.Update(c.Context, name, vm.UpdateOptions{MaxRunHours: c.Int("max-hours")})
}

func clusterStart(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArg(c, 0, "cluster name")
	if err != nil {
		return err
	}
	if !c.IsSet("num-instances") || c.Int("num-instances") <= 0 {
		return fmt.Errorf("--num-instances must be a positive integer")
	}
	announce(state)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	if err := service.Start(c.Context, clusterName, cluster.StartOptions{NumInstances: c.Int("num-instances")}); err != nil {
		return err
	}
	if state.State != nil {
		if err := state.State.RecordClusterStart(clusterName, state.Profile, c.Int("num-instances"), time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}
	return nil
}

func clusterStop(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArg(c, 0, "cluster name")
	if err != nil {
		return err
	}
	announce(state)
	if !c.Bool("delete") && c.Bool("keep-disks") {
		return fmt.Errorf("--keep-disks requires --delete")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	if err := service.Stop(c.Context, clusterName, cluster.StopOptions{
		Delete:    c.Bool("delete"),
		KeepDisks: c.Bool("keep-disks"),
	}); err != nil {
		return err
	}
	if state.State != nil {
		if err := state.State.RecordClusterStop(clusterName, c.Bool("delete"), time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}
	return nil
}

func clusterStatus(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArg(c, 0, "cluster name")
	if err != nil {
		return err
	}
	announce(state)
	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	return service.Show(c.Context, clusterName)
}

func clusterUpdate(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArg(c, 0, "cluster name")
	if err != nil {
		return err
	}
	announce(state)
	if !c.IsSet("max-hours") || c.Int("max-hours") <= 0 {
		return fmt.Errorf("--max-hours must be a positive integer")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	if err := service.Update(c.Context, clusterName, cluster.UpdateOptions{MaxRunHours: c.Int("max-hours")}); err != nil {
		return err
	}
	if state.State != nil {
		if err := state.State.RecordClusterUpdate(clusterName, time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}
	return nil
}

func sshAction(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	targetRaw := c.Args().Get(0)
	targetProvided := targetRaw != ""
	if !targetProvided {
		targetRaw = state.Config.VM.DefaultName
	}
	announce(state)

	user := c.String("user")
	if user == "" {
		user = state.Config.SSH.DefaultUser
	}
	if user == "" {
		return fmt.Errorf("ssh user is required (set --user or ssh.default_user)")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	resolved, err := ssh.ResolveTarget(c.Context, compute, state.Config, targetRaw)
	if err != nil {
		return err
	}

	proxy := ""
	if resolved.Index > 0 {
		proxy = ssh.FormatUserHost(user, resolved.MasterPublicIP)
	}

	commandArgs := sshCommandArgs(c.Args().Slice(), targetProvided)
	args := ssh.BuildSSHArgs(ssh.SSHOptions{
		User:         user,
		Host:         resolved.Host,
		ProxyJump:    proxy,
		ForwardAgent: true,
		Command:      commandArgs,
	})

	state.UI.Detailf(1, "cmd: %s", formatCommand("ssh", args))
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func scpAction(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	announce(state)

	user := c.String("user")
	if user == "" {
		user = state.Config.SSH.DefaultUser
	}
	if user == "" {
		return fmt.Errorf("scp user is required (set --user or ssh.default_user)")
	}

	flags, srcArg, dstArg, err := parseScpArgs(c.Args().Slice())
	if err != nil {
		return err
	}

	srcSpec, err := ssh.ParseRemoteSpec(srcArg)
	if err != nil {
		return err
	}
	dstSpec, err := ssh.ParseRemoteSpec(dstArg)
	if err != nil {
		return err
	}

	if srcSpec.IsRemote && dstSpec.IsRemote {
		return fmt.Errorf("either src or dst must be local")
	}
	if !srcSpec.IsRemote && !dstSpec.IsRemote {
		return fmt.Errorf("either src or dst must be remote")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	var proxy string
	var src string
	var dst string

	if srcSpec.IsRemote {
		resolved, err := ssh.ResolveClusterTarget(c.Context, compute, state.Config, srcSpec.Target.Raw)
		if err != nil {
			return err
		}
		if resolved.Index > 0 {
			proxy = ssh.FormatUserHost(user, resolved.MasterPublicIP)
		}
		src = fmt.Sprintf("%s:%s", ssh.FormatUserHost(user, resolved.Host), ssh.NormalizePath(srcSpec.Path))
		dst = dstArg
	} else {
		resolved, err := ssh.ResolveClusterTarget(c.Context, compute, state.Config, dstSpec.Target.Raw)
		if err != nil {
			return err
		}
		if resolved.Index > 0 {
			proxy = ssh.FormatUserHost(user, resolved.MasterPublicIP)
		}
		src = srcArg
		dst = fmt.Sprintf("%s:%s", ssh.FormatUserHost(user, resolved.Host), ssh.NormalizePath(dstSpec.Path))
	}

	args := append([]string{}, flags...)
	args = append(args, ssh.BuildSCPArgs(ssh.SCPOptions{
		ProxyJump: proxy,
		Src:       src,
		Dst:       dst,
	})...)

	state.UI.Detailf(1, "cmd: %s", formatCommand("scp", args))
	cmd := exec.Command("scp", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveVMTarget(c *cli.Context, state *State) (target.Target, error) {
	if c.Args().Len() == 0 {
		return target.Target{Raw: state.Config.VM.DefaultName, Name: state.Config.VM.DefaultName}, nil
	}
	raw := c.Args().First()
	parsed, err := target.Parse(raw)
	if err != nil {
		return target.Target{}, err
	}
	return parsed, nil
}

func requireArg(c *cli.Context, index int, label string) (string, error) {
	if c.Args().Len() <= index {
		return "", fmt.Errorf("%s is required", label)
	}
	value := strings.TrimSpace(c.Args().Get(index))
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}
