// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	skipPush   bool
	skipDeploy bool
	watch      bool
)

func init() {

	upFlagset := pflag.NewFlagSet("up", pflag.ExitOnError)
	upFlagset.BoolVar(&skipPush, "skip-push", false, "Skip pushing function to remote registry")
	upFlagset.BoolVar(&skipDeploy, "skip-deploy", false, "Skip function deployment")
	upFlagset.StringVar(&remoteBuilder, "remote-builder", "", "URL to the builder")
	upFlagset.StringVar(&payloadSecretPath, "payload-secret", "", "Path to payload secret file")

	upFlagset.BoolVar(&watch, "watch", false, "Watch for changes in files and re-deploy")
	upCmd.Flags().AddFlagSet(upFlagset)

	build, _, _ := faasCmd.Find([]string{"build"})
	upCmd.Flags().AddFlagSet(build.Flags())

	push, _, _ := faasCmd.Find([]string{"push"})
	upCmd.Flags().AddFlagSet(push.Flags())

	deploy, _, _ := faasCmd.Find([]string{"deploy"})
	upCmd.Flags().AddFlagSet(deploy.Flags())

	faasCmd.AddCommand(upCmd)
}

// upCmd is a wrapper to the build, push and deploy commands
var upCmd = &cobra.Command{
	Use:   `up -f [YAML_FILE] [--skip-push] [--skip-deploy] [flags from build, push, deploy]`,
	Short: "Builds, pushes and deploys OpenFaaS function containers",
	Long: `Build, Push, and Deploy OpenFaaS function containers either via the
supplied YAML config using the "--yaml" flag (which may contain multiple function
definitions), or directly via flags.

The push step may be skipped by setting the --skip-push flag
and the deploy step with --skip-deploy.

Note: All flags from the build, push and deploy flags are valid and can be combined,
see the --help text for those commands for details.`,
	Example: `  faas-cli up -f myfn.yaml
faas-cli up --filter "*gif*" --secret dockerhuborg`,
	PreRunE: preRunUp,
	RunE:    upHandler,
}

func preRunUp(cmd *cobra.Command, args []string) error {
	if err := preRunBuild(cmd, args); err != nil {
		return err
	}
	if err := preRunDeploy(cmd, args); err != nil {
		return err
	}
	return nil
}

func upHandler(cmd *cobra.Command, args []string) error {
	if watch {
		return watchLoop(cmd, args, func(cmd *cobra.Command, args []string, ctx context.Context) error {

			fmt.Println("[Watch] Change a file to trigger a rebuild...")

			return upRunner(cmd, args, ctx)
		})
	}

	ctx := context.Background()
	return upRunner(cmd, args, ctx)
}

func upRunner(cmd *cobra.Command, args []string, ctx context.Context) error {
	if err := runBuild(cmd, args); err != nil {
		return err
	}

	if !skipPush && remoteBuilder == "" {
		if err := runPush(cmd, args); err != nil {
			return err
		}
	}

	if !skipDeploy {
		if err := runDeploy(cmd, args); err != nil {
			return err
		}
	}

	return nil
}

func ignorePatterns() ([]gitignore.Pattern, error) {
	gitignorePath := ".gitignore"

	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	patterns := []gitignore.Pattern{gitignore.ParsePattern(".git", nil)}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}
