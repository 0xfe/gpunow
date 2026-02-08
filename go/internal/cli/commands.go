package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"gpunow/internal/cluster"
	"gpunow/internal/config"
	"gpunow/internal/gcp"
	"gpunow/internal/ssh"
	appstate "gpunow/internal/state"
	"gpunow/internal/target"
	"gpunow/internal/version"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:                   "gpunow",
		Usage:                  "Manage GPU clusters on GCP",
		Version:                version.Version,
		UseShortOptionHandling: true,
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
			configCommand(),
			createCommand(),
			startCommand(),
			stopCommand(),
			updateCommand(),
			sshCommand(),
			scpCommand(),
			statusCommand(),
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

func createCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a cluster",
		ArgsUsage: "<cluster>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "num-instances", Aliases: []string{"n"}, Usage: "Number of instances"},
			&cli.BoolFlag{Name: "start", Usage: "Start the cluster after creating it"},
			&cli.BoolFlag{Name: "estimate-cost", Usage: "Estimate creation cost before proceeding"},
			&cli.BoolFlag{Name: "refresh", Usage: "Refresh cached pricing data (requires --estimate-cost)"},
			&cli.StringFlag{Name: "gcp-machine-type", Usage: "Override machine type for this cluster"},
			&cli.IntFlag{Name: "gcp-max-run-hours", Usage: "Override max run duration in hours for this cluster"},
			&cli.StringFlag{Name: "gcp-termination-action", Usage: "Override termination action (DELETE|STOP) for this cluster"},
			&cli.IntFlag{Name: "gcp-disk-size-gb", Usage: "Override boot disk size in GB for this cluster"},
			&cli.BoolFlag{Name: "keep-disks", Usage: "Preserve boot disks on delete for this cluster"},
		},
		Action: createCluster,
	}
}

func startCommand() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "Start a cluster",
		ArgsUsage: "<cluster>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "num-instances", Aliases: []string{"n"}, Usage: "Number of instances (required to create new clusters)"},
		},
		Action: startCluster,
	}
}

func stopCommand() *cli.Command {
	return &cli.Command{
		Name:      "stop",
		Usage:     "Stop or delete a cluster",
		ArgsUsage: "<cluster>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "delete", Usage: "Delete instances"},
			&cli.BoolFlag{Name: "delete-disks", Usage: "Delete boot disks when deleting instances"},
		},
		Action: stopCluster,
	}
}

func updateCommand() *cli.Command {
	return &cli.Command{
		Name:      "update",
		Usage:     "Update cluster settings",
		ArgsUsage: "<cluster>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "max-hours", Usage: "Max run duration in hours"},
		},
		Action: updateCluster,
	}
}

func sshCommand() *cli.Command {
	return &cli.Command{
		Name:      "ssh",
		Usage:     "SSH into a cluster node",
		ArgsUsage: "<cluster/index>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "user", Aliases: []string{"u"}, Usage: "SSH username"},
		},
		Action: sshAction,
	}
}

func scpCommand() *cli.Command {
	return &cli.Command{
		Name:      "scp",
		Usage:     "SCP files to/from a cluster node",
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

func createCluster(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArgWithHelp(c, 0, "cluster name")
	if err != nil {
		return err
	}
	numInstances, numInstancesExplicit, err := parseNumInstancesValue(c)
	if err != nil {
		return usageError(c, err.Error())
	}
	if !numInstancesExplicit || numInstances <= 0 {
		return usageError(c, "--num-instances must be a positive integer")
	}
	clusterConfig, err := parseCreateClusterConfig(c)
	if err != nil {
		return usageError(c, err.Error())
	}
	startNow := c.Bool("start") || hasBoolArg(c.Args().Slice(), "start")
	estimateCost := c.Bool("estimate-cost") || hasBoolArg(c.Args().Slice(), "estimate-cost")
	refreshPricing := c.Bool("refresh") || hasBoolArg(c.Args().Slice(), "refresh")
	if refreshPricing && !estimateCost {
		return usageError(c, "--refresh requires --estimate-cost")
	}
	if startNow {
		return createAndStartCluster(c, state, clusterName, numInstances, createOptions{
			ClusterConfig:  clusterConfig,
			EstimateCost:   estimateCost,
			RefreshPricing: refreshPricing,
		})
	}
	announce(state)
	if estimateCost {
		compute, err := state.ComputeClient(c.Context)
		if err != nil {
			return err
		}
		if err := estimateClusterCreateCost(c.Context, state, compute, numInstances, refreshPricing, clusterConfig); err != nil {
			return err
		}
	}
	if state.State != nil {
		if err := state.State.RecordClusterCreate(clusterName, state.Profile, numInstances, clusterConfig, time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		} else {
			state.UI.Successf("Created cluster %s (%d instances) in local state", clusterName, numInstances)
		}
		return nil
	}
	state.UI.Successf("Created cluster %s (%d instances)", clusterName, numInstances)
	return nil
}

type createOptions struct {
	ClusterConfig  appstate.ClusterConfig
	EstimateCost   bool
	RefreshPricing bool
}

func createAndStartCluster(c *cli.Context, state *State, clusterName string, numInstances int, opts createOptions) error {
	selection, err := resolveSSHSelection(state)
	if err != nil {
		return err
	}
	user := strings.TrimSpace(state.Config.SSH.DefaultUser)
	if selection != nil && selection.Key != "" && user == "" {
		return fmt.Errorf("ssh.default_user is required to set ssh keys")
	}
	announceWithKey(state, selection, true)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}
	if opts.EstimateCost {
		if err := estimateClusterCreateCost(c.Context, state, compute, numInstances, opts.RefreshPricing, opts.ClusterConfig); err != nil {
			return err
		}
	}

	if state.State != nil {
		if err := state.State.RecordClusterCreate(clusterName, state.Profile, numInstances, opts.ClusterConfig, time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	startOptions := applyClusterConfig(cluster.StartOptions{
		NumInstances: numInstances,
		SSHUser:      user,
		SSHPublicKey: selectionKey(selection),
	}, opts.ClusterConfig)
	if err := service.Start(c.Context, clusterName, startOptions); err != nil {
		return err
	}
	if state.State != nil {
		if err := state.State.RecordClusterStart(clusterName, state.Profile, numInstances, opts.ClusterConfig, time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}
	return nil
}

func startCluster(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArgWithHelp(c, 0, "cluster name")
	if err != nil {
		return err
	}
	numInstances, numInstancesExplicit, err := parseNumInstancesValue(c)
	if err != nil {
		return usageError(c, err.Error())
	}
	if numInstancesExplicit && numInstances <= 0 {
		return usageError(c, "--num-instances must be a positive integer")
	}
	clusterConfig := appstate.ClusterConfig{}
	var clusterEntryNumInstances int
	if state.State != nil {
		data, err := state.State.Load()
		if err != nil {
			return err
		}
		entry := data.Clusters[clusterName]
		if entry == nil {
			return usageError(c, fmt.Sprintf("cluster %s not found in state; run `gpunow create %s -n <num>` first", clusterName, clusterName))
		}
		clusterEntryNumInstances = entry.NumInstances
		clusterConfig = entry.Config
	}
	if !numInstancesExplicit {
		numInstances = clusterEntryNumInstances
		if numInstances <= 0 {
			return usageError(c, fmt.Sprintf("cluster %s has no instance count in state; run `gpunow create %s -n <num>`", clusterName, clusterName))
		}
	}
	selection, err := resolveSSHSelection(state)
	if err != nil {
		return err
	}
	user := strings.TrimSpace(state.Config.SSH.DefaultUser)
	if selection != nil && selection.Key != "" && user == "" {
		return fmt.Errorf("ssh.default_user is required to set ssh keys")
	}
	announceWithKey(state, selection, true)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	startOptions := applyClusterConfig(cluster.StartOptions{
		NumInstances: numInstances,
		SSHUser:      user,
		SSHPublicKey: selectionKey(selection),
	}, clusterConfig)
	if err := service.Start(c.Context, clusterName, startOptions); err != nil {
		return err
	}
	if state.State != nil {
		if err := state.State.RecordClusterStart(clusterName, state.Profile, numInstances, clusterConfig, time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}
	return nil
}

func stopCluster(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArgWithHelp(c, 0, "cluster name")
	if err != nil {
		return err
	}
	announce(state)
	deleteFlag := c.Bool("delete") || hasBoolArg(c.Args().Slice(), "delete")
	deleteDisks := c.Bool("delete-disks") || hasBoolArg(c.Args().Slice(), "delete-disks")
	if !deleteFlag && deleteDisks {
		return usageError(c, "--delete-disks requires --delete")
	}
	clusterConfig := appstate.ClusterConfig{}
	if state.State != nil {
		data, err := state.State.Load()
		if err != nil {
			return err
		}
		if entry := data.Clusters[clusterName]; entry != nil {
			clusterConfig = entry.Config
		}
	}
	keepDisks := clusterConfig.KeepDisks && !deleteDisks

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	if err := service.Stop(c.Context, clusterName, cluster.StopOptions{
		Delete:      deleteFlag,
		KeepDisks:   keepDisks,
		DeleteDisks: deleteDisks,
	}); err != nil {
		return err
	}
	if state.State != nil {
		if deleteFlag {
			if err := state.State.DeleteCluster(clusterName); err != nil {
				state.UI.Warnf("Failed to update state: %v", err)
			}
		} else if err := state.State.RecordClusterStop(clusterName, deleteFlag, time.Now()); err != nil {
			state.UI.Warnf("Failed to update state: %v", err)
		}
	}
	return nil
}

func updateCluster(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	clusterName, err := requireArgWithHelp(c, 0, "cluster name")
	if err != nil {
		return err
	}
	announce(state)
	maxHours, maxHoursExplicit, err := parseMaxHoursValue(c)
	if err != nil {
		return usageError(c, err.Error())
	}
	if !maxHoursExplicit || maxHours <= 0 {
		return usageError(c, "--max-hours must be a positive integer")
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
	if err := service.Update(c.Context, clusterName, cluster.UpdateOptions{MaxRunHours: maxHours}); err != nil {
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
	targetRaw, err := requireArgWithHelp(c, 0, "target")
	if err != nil {
		return err
	}
	user := c.String("user")
	if user == "" {
		user = state.Config.SSH.DefaultUser
	}
	if user == "" {
		return fmt.Errorf("ssh user is required (set --user or ssh.default_user)")
	}
	selection, err := resolveSSHSelection(state)
	if err != nil {
		return err
	}
	if selection != nil && selection.Key != "" && strings.TrimSpace(user) == "" {
		return fmt.Errorf("ssh.default_user is required to set ssh keys")
	}
	announceWithKey(state, selection, true)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	targetSpec, err := target.Parse(targetRaw)
	if err != nil {
		return err
	}
	if !targetSpec.IsCluster {
		return fmt.Errorf("target must be cluster/index (foo/0 or foo-0)")
	}

	resolved, err := ssh.ResolveClusterTarget(c.Context, compute, state.Config, targetRaw)
	if err != nil {
		return err
	}

	publicKey := selectionKey(selection)
	if publicKey != "" {
		if err := ensureSSHKeysForTarget(c.Context, compute, state.Config, user, publicKey, targetSpec); err != nil {
			return err
		}
	}

	commandArgs := sshCommandArgs(c.Args().Slice(), true)
	identityFile := ""
	if selection != nil {
		identityFile = selection.IdentityPath
	}
	args := ssh.BuildSSHArgs(ssh.SSHOptions{
		User:         user,
		Host:         resolved.Host,
		ProxyJump:    "",
		ForwardAgent: true,
		IdentityFile: identityFile,
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
	user := c.String("user")
	if user == "" {
		user = state.Config.SSH.DefaultUser
	}
	if user == "" {
		return fmt.Errorf("scp user is required (set --user or ssh.default_user)")
	}
	selection, err := resolveSSHSelection(state)
	if err != nil {
		return err
	}
	if selection != nil && selection.Key != "" && strings.TrimSpace(user) == "" {
		return fmt.Errorf("ssh.default_user is required to set ssh keys")
	}
	announceWithKey(state, selection, true)

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

	var src string
	var dst string
	publicKey := selectionKey(selection)

	if srcSpec.IsRemote {
		resolved, err := ssh.ResolveClusterTarget(c.Context, compute, state.Config, srcSpec.Target.Raw)
		if err != nil {
			return err
		}
		if publicKey != "" {
			if err := ensureSSHKeysForTarget(c.Context, compute, state.Config, user, publicKey, srcSpec.Target); err != nil {
				return err
			}
		}
		src = fmt.Sprintf("%s:%s", ssh.FormatUserHost(user, resolved.Host), ssh.NormalizePath(srcSpec.Path))
		dst = dstArg
	} else {
		resolved, err := ssh.ResolveClusterTarget(c.Context, compute, state.Config, dstSpec.Target.Raw)
		if err != nil {
			return err
		}
		if publicKey != "" {
			if err := ensureSSHKeysForTarget(c.Context, compute, state.Config, user, publicKey, dstSpec.Target); err != nil {
				return err
			}
		}
		src = srcArg
		dst = fmt.Sprintf("%s:%s", ssh.FormatUserHost(user, resolved.Host), ssh.NormalizePath(dstSpec.Path))
	}

	identityFile := ""
	if selection != nil {
		identityFile = selection.IdentityPath
	}
	args := append([]string{}, flags...)
	args = append(args, ssh.BuildSCPArgs(ssh.SCPOptions{
		ProxyJump:    "",
		IdentityFile: identityFile,
		Src:          src,
		Dst:          dst,
	})...)

	state.UI.Detailf(1, "cmd: %s", formatCommand("scp", args))
	cmd := exec.Command("scp", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveSSHSelection(state *State) (*ssh.PublicKeySelection, error) {
	selection, err := ssh.ResolvePublicKeySelection(state.Config.SSH.IdentityFile)
	if err != nil {
		return nil, err
	}
	return selection, nil
}

func selectionKey(selection *ssh.PublicKeySelection) string {
	if selection == nil {
		return ""
	}
	return selection.Key
}

func ensureSSHKeysForTarget(ctx context.Context, compute gcp.Compute, cfg *config.Config, user string, publicKey string, targetSpec target.Target) error {
	if publicKey == "" {
		return nil
	}
	if err := ssh.EnsureInstanceSSHKey(ctx, compute, cfg, targetSpec.Name, user, publicKey); err != nil {
		return fmt.Errorf("ensure ssh key on %s: %w", targetSpec.Name, err)
	}
	return nil
}

func requireArgWithHelp(c *cli.Context, index int, label string) (string, error) {
	value, err := requireArg(c, index, label)
	if err != nil {
		showCommandUsage(c)
	}
	return value, err
}

func usageError(c *cli.Context, msg string) error {
	showCommandUsage(c)
	return fmt.Errorf("%s", msg)
}

func showCommandUsage(c *cli.Context) {
	if c == nil || c.Command == nil {
		return
	}
	origWriter := c.App.Writer
	if c.App.ErrWriter != nil {
		c.App.Writer = c.App.ErrWriter
		defer func() { c.App.Writer = origWriter }()
	}
	if err := cli.ShowCommandHelp(c, c.Command.Name); err == nil {
		return
	}
	if err := cli.ShowSubcommandHelp(c); err == nil {
		return
	}
	_ = cli.ShowAppHelp(c)
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

func hasBoolArg(args []string, name string) bool {
	flag := "--" + name
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func parseNumInstancesValue(c *cli.Context) (int, bool, error) {
	return parseIntFlagValue(c, "num-instances", "-n", "--num-instances", "--num-instances must be a positive integer")
}

func parseCreateClusterConfig(c *cli.Context) (appstate.ClusterConfig, error) {
	clusterConfig := appstate.ClusterConfig{}

	machineType, machineTypeSet, err := parseStringFlagValue(c, "--gcp-machine-type", "gcp-machine-type")
	if err != nil {
		return appstate.ClusterConfig{}, err
	}
	if machineTypeSet {
		value := strings.TrimSpace(machineType)
		if value == "" {
			return appstate.ClusterConfig{}, fmt.Errorf("--gcp-machine-type cannot be empty")
		}
		clusterConfig.GCPMachineType = value
	}

	maxRunHours, maxRunHoursSet, err := parseIntFlagValue(c, "gcp-max-run-hours", "", "--gcp-max-run-hours", "--gcp-max-run-hours must be a positive integer")
	if err != nil {
		return appstate.ClusterConfig{}, err
	}
	if maxRunHoursSet {
		value := maxRunHours
		if value <= 0 {
			return appstate.ClusterConfig{}, fmt.Errorf("--gcp-max-run-hours must be a positive integer")
		}
		clusterConfig.GCPMaxRunHours = value
	}

	terminationAction, terminationActionSet, err := parseStringFlagValue(c, "--gcp-termination-action", "gcp-termination-action")
	if err != nil {
		return appstate.ClusterConfig{}, err
	}
	if terminationActionSet {
		value := strings.ToUpper(strings.TrimSpace(terminationAction))
		if value != "DELETE" && value != "STOP" {
			return appstate.ClusterConfig{}, fmt.Errorf("--gcp-termination-action must be DELETE or STOP")
		}
		clusterConfig.GCPTerminationAction = value
	}

	diskSizeGB, diskSizeGBSet, err := parseIntFlagValue(c, "gcp-disk-size-gb", "", "--gcp-disk-size-gb", "--gcp-disk-size-gb must be a positive integer")
	if err != nil {
		return appstate.ClusterConfig{}, err
	}
	if diskSizeGBSet {
		value := diskSizeGB
		if value <= 0 {
			return appstate.ClusterConfig{}, fmt.Errorf("--gcp-disk-size-gb must be a positive integer")
		}
		clusterConfig.GCPDiskSizeGB = value
	}
	clusterConfig.KeepDisks = c.Bool("keep-disks") || hasBoolArg(c.Args().Slice(), "keep-disks")
	return clusterConfig, nil
}

func applyClusterConfig(startOptions cluster.StartOptions, clusterConfig appstate.ClusterConfig) cluster.StartOptions {
	if machineType := strings.TrimSpace(clusterConfig.GCPMachineType); machineType != "" {
		startOptions.MachineType = machineType
	}
	if clusterConfig.GCPMaxRunHours > 0 {
		startOptions.MaxRunHours = clusterConfig.GCPMaxRunHours
	}
	if terminationAction := strings.TrimSpace(clusterConfig.GCPTerminationAction); terminationAction != "" {
		startOptions.TerminationAction = terminationAction
	}
	if clusterConfig.GCPDiskSizeGB > 0 {
		startOptions.DiskSizeGB = clusterConfig.GCPDiskSizeGB
	}
	startOptions.KeepDisks = clusterConfig.KeepDisks
	return startOptions
}

func parseMaxHoursValue(c *cli.Context) (int, bool, error) {
	return parseIntFlagValue(c, "max-hours", "", "--max-hours", "--max-hours must be a positive integer")
}

func parseIntFlagValue(c *cli.Context, flagName, shortFlag, longFlag, errMsg string) (int, bool, error) {
	if c.IsSet(flagName) {
		return c.Int(flagName), true, nil
	}
	return parseIntFlagFromArgs(c.Args().Slice(), shortFlag, longFlag, errMsg)
}

func parseStringFlagValue(c *cli.Context, longFlag, flagName string) (string, bool, error) {
	if c.IsSet(flagName) {
		return c.String(flagName), true, nil
	}
	return parseStringFlagFromArgs(c.Args().Slice(), longFlag)
}

func parseIntFlagFromArgs(args []string, shortFlag string, longFlag string, errMsg string) (int, bool, error) {
	found := false
	value := 0
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == shortFlag || arg == longFlag:
			if idx+1 >= len(args) {
				return 0, true, fmt.Errorf("%s", errMsg)
			}
			parsed, err := strconv.Atoi(args[idx+1])
			if err != nil {
				return 0, true, fmt.Errorf("%s", errMsg)
			}
			found = true
			value = parsed
			idx++
		case strings.HasPrefix(arg, longFlag+"="):
			parsed, err := strconv.Atoi(strings.TrimPrefix(arg, longFlag+"="))
			if err != nil {
				return 0, true, fmt.Errorf("%s", errMsg)
			}
			found = true
			value = parsed
		case shortFlag != "" && strings.HasPrefix(arg, shortFlag) && len(arg) > len(shortFlag):
			parsed, err := strconv.Atoi(arg[len(shortFlag):])
			if err != nil {
				return 0, true, fmt.Errorf("%s", errMsg)
			}
			found = true
			value = parsed
		}
	}
	return value, found, nil
}

func parseStringFlagFromArgs(args []string, longFlag string) (string, bool, error) {
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == longFlag:
			if idx+1 >= len(args) {
				return "", true, fmt.Errorf("%s requires a value", longFlag)
			}
			return args[idx+1], true, nil
		case strings.HasPrefix(arg, longFlag+"="):
			return strings.TrimPrefix(arg, longFlag+"="), true, nil
		}
	}
	return "", false, nil
}
