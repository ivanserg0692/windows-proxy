package wsl

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"wslproxy/internal/config"
)

type Client struct{ Runner Runner }

func (c Client) Distributions(ctx context.Context) ([]string, error) {
	data, err := c.Runner.Run(ctx, "--list", "--quiet")
	if err != nil {
		return nil, err
	}
	output, err := decodeOutput(data)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if err := config.ValidateDistribution(name); err != nil {
			return nil, fmt.Errorf("invalid distribution name in WSL output")
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names, nil
}

func (c Client) CheckDistribution(ctx context.Context, name string) error {
	if err := config.ValidateDistribution(name); err != nil {
		return err
	}
	names, err := c.Distributions(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(names, name) {
		return fmt.Errorf("WSL distribution %q is not installed; select another distribution", name)
	}
	return nil
}
