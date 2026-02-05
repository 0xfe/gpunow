package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"gpunow/internal/config"
	"gpunow/internal/gcp"
	"gpunow/internal/home"
	"gpunow/internal/logging"
	appstate "gpunow/internal/state"
	"gpunow/internal/ui"
)

type State struct {
	Config  *config.Config
	UI      *ui.UI
	Logger  *zap.Logger
	Profile string
	Home    home.Home
	State   *appstate.Store
	Compute gcp.Compute
}

const stateKey = "state"

func GetState(c *cli.Context) (*State, error) {
	if c.App.Metadata == nil {
		c.App.Metadata = map[string]any{}
	}
	if existing, ok := c.App.Metadata[stateKey].(*State); ok {
		return existing, nil
	}

	profile := c.String("profile")
	uiPrinter := ui.New()

	logger, err := logging.New(c.String("log-level"))
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	resolvedHome, err := home.Resolve()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(profile, resolvedHome.ProfilesDir)
	if err != nil {
		return nil, err
	}

	state := &State{
		Config:  cfg,
		UI:      uiPrinter,
		Logger:  logger,
		Profile: profile,
		Home:    resolvedHome,
		State:   appstate.New(resolvedHome.StateDir),
	}
	c.App.Metadata[stateKey] = state
	return state, nil
}

func (s *State) ComputeClient(ctx context.Context) (gcp.Compute, error) {
	if s.Compute != nil {
		return s.Compute, nil
	}
	client, err := gcp.New(ctx)
	if err != nil {
		return nil, err
	}
	s.Compute = client
	return s.Compute, nil
}
