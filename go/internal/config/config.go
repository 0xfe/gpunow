package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"

	"gpunow/internal/validate"
)

type Config struct {
	Project        ProjectConfig        `toml:"project" validate:"required"`
	Cluster        ClusterConfig        `toml:"cluster" validate:"required"`
	VM             VMConfig             `toml:"vm" validate:"required"`
	Instance       InstanceConfig       `toml:"instance" validate:"required"`
	Network        NetworkConfig        `toml:"network" validate:"required"`
	GPU            GPUConfig            `toml:"gpu" validate:"required"`
	Disk           DiskConfig           `toml:"disk" validate:"required"`
	ServiceAccount ServiceAccountConfig `toml:"service_account" validate:"required"`
	Shielded       ShieldedConfig       `toml:"shielded"`
	Labels         map[string]string    `toml:"labels"`
	Reservation    ReservationConfig    `toml:"reservation"`
	SSH            SSHConfig            `toml:"ssh"`
	Files          FilesConfig          `toml:"files" validate:"required"`
	Metadata       map[string]string    `toml:"metadata"`
	Paths          Paths                `toml:"-"`
	Profile        string               `toml:"-"`
}

type Paths struct {
	Dir              string
	ConfigFile       string
	CloudInitFile    string
	SetupScript      string
	ZshrcFile        string
	ProfilesBasePath string
}

type ProjectConfig struct {
	ID   string `toml:"id" validate:"required"`
	Zone string `toml:"zone" validate:"required"`
}

type ClusterConfig struct {
	NetworkNamePrefix string `toml:"network_name_prefix" validate:"required"`
	SubnetCIDRBase    string `toml:"subnet_cidr_base" validate:"required,cidr"`
	SubnetPrefix      int    `toml:"subnet_prefix" validate:"gte=8,lte=30"`
}

type VMConfig struct {
	DefaultName string `toml:"default_name" validate:"required"`
}

type InstanceConfig struct {
	MachineType         string `toml:"machine_type" validate:"required"`
	ProvisioningModel   string `toml:"provisioning_model" validate:"required,oneof=SPOT STANDARD"`
	MaintenancePolicy   string `toml:"maintenance_policy" validate:"required,oneof=TERMINATE MIGRATE"`
	TerminationAction   string `toml:"termination_action" validate:"required,oneof=DELETE STOP"`
	MaxRunHours         int    `toml:"max_run_hours" validate:"gt=0"`
	RestartOnFailure    bool   `toml:"restart_on_failure"`
	KeyRevocationAction string `toml:"key_revocation_action" validate:"required"`
	HostnameDomain      string `toml:"hostname_domain"`
}

type NetworkConfig struct {
	DefaultNetwork string   `toml:"default_network" validate:"required"`
	StackType      string   `toml:"stack_type" validate:"required"`
	NetworkTier    string   `toml:"network_tier" validate:"required"`
	Ports          []int    `toml:"ports" validate:"min=1,dive,gt=0,lte=65535"`
	TagsBase       []string `toml:"tags_base" validate:"min=1,dive,required"`
}

type GPUConfig struct {
	Type  string `toml:"type" validate:"required"`
	Count int    `toml:"count" validate:"gt=0"`
}

type DiskConfig struct {
	Boot       bool   `toml:"boot"`
	AutoDelete bool   `toml:"auto_delete"`
	SizeGB     int    `toml:"size_gb" validate:"gt=0"`
	Type       string `toml:"type" validate:"required"`
	Mode       string `toml:"mode" validate:"required"`
	Image      string `toml:"image" validate:"required"`
}

type ServiceAccountConfig struct {
	Email  string   `toml:"email" validate:"required"`
	Scopes []string `toml:"scopes" validate:"min=1,dive,required"`
}

type ShieldedConfig struct {
	SecureBoot          bool `toml:"secure_boot"`
	VTPM                bool `toml:"vtpm"`
	IntegrityMonitoring bool `toml:"integrity_monitoring"`
}

type ReservationConfig struct {
	Affinity string `toml:"affinity"`
}

type SSHConfig struct {
	DefaultUser string `toml:"default_user"`
}

type FilesConfig struct {
	CloudInit   string `toml:"cloud_init" validate:"required"`
	SetupScript string `toml:"setup_script" validate:"required"`
	Zshrc       string `toml:"zshrc" validate:"required"`
}

func Load(profile string, baseDir string) (*Config, error) {
	if baseDir == "" {
		baseDir = "profiles"
	}
	if profile == "" {
		profile = "default"
	}

	cfgDir := filepath.Join(baseDir, profile)
	cfgPath := filepath.Join(cfgDir, "config.toml")

	if _, err := os.Stat(cfgPath); err != nil {
		return nil, fmt.Errorf("profile not found: %s", cfgPath)
	}

	var cfg Config
	if _, err := toml.DecodeFile(cfgPath, &cfg); err != nil {
		return nil, fmt.Errorf("parse config.toml: %w", err)
	}

	applyDefaults(&cfg)
	cfg.Profile = profile
	cfg.Paths = Paths{
		Dir:              cfgDir,
		ConfigFile:       cfgPath,
		CloudInitFile:    filepath.Join(cfgDir, cfg.Files.CloudInit),
		SetupScript:      filepath.Join(cfgDir, cfg.Files.SetupScript),
		ZshrcFile:        filepath.Join(cfgDir, cfg.Files.Zshrc),
		ProfilesBasePath: baseDir,
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.VM.DefaultName == "" {
		cfg.VM.DefaultName = "gpu0"
	}
	if cfg.Files.CloudInit == "" {
		cfg.Files.CloudInit = "cloud-init.yaml"
	}
	if cfg.Files.SetupScript == "" {
		cfg.Files.SetupScript = "setup.sh"
	}
	if cfg.Files.Zshrc == "" {
		cfg.Files.Zshrc = "zshrc"
	}
}

func validateConfig(cfg *Config) error {
	v := validator.New()
	if err := v.RegisterValidation("cidr", validateCIDR); err != nil {
		return err
	}
	if err := v.Struct(cfg); err != nil {
		return formatValidationError(err)
	}

	if !validate.IsResourceName(cfg.VM.DefaultName) {
		return fmt.Errorf("vm.default_name must be a valid resource name")
	}
	if !validate.IsResourceName(cfg.Cluster.NetworkNamePrefix) {
		return fmt.Errorf("cluster.network_name_prefix must be a valid resource name")
	}
	if !validate.IsResourceName(cfg.Network.DefaultNetwork) {
		return fmt.Errorf("network.default_network must be a valid resource name")
	}
	if cfg.Instance.HostnameDomain != "" && !validate.IsHostnameDomain(cfg.Instance.HostnameDomain) {
		return fmt.Errorf("instance.hostname_domain must be a valid DNS domain like example.com")
	}

	if err := ensureFile(cfg.Paths.CloudInitFile); err != nil {
		return fmt.Errorf("cloud-init file: %w", err)
	}
	if err := ensureFile(cfg.Paths.SetupScript); err != nil {
		return fmt.Errorf("setup script: %w", err)
	}
	if err := ensureFile(cfg.Paths.ZshrcFile); err != nil {
		return fmt.Errorf("zshrc file: %w", err)
	}

	return nil
}

func validateCIDR(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	_, _, err := net.ParseCIDR(value)
	return err == nil
}

func ensureFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("expected file, got directory: %s", path)
	}
	return nil
}

func formatValidationError(err error) error {
	if err == nil {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("invalid config:\n")
	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			fmt.Fprintf(&sb, "- %s failed %s\n", e.Namespace(), e.Tag())
		}
		return errors.New(sb.String())
	}
	return err
}
