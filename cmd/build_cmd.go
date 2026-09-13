package cmd

import (
	"context"
	"errors"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
)

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds current directory",
	RunE:  runBuild,
}

func runBuild(cmd *cobra.Command, args []string) (err error) {
	path, err := conf.ResolvePath(".")
	if err != nil {
		return err
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer func() { err = errors.Join(err, sess.Close()) }()

	_, err = sess.Build(ctx, engine.Local{ConfigPath: path}, args, newObserver())
	return err
}
