package main

import (
	_ "embed"
	"fmt"
	"hyperfocus/app/cmd"
	"hyperfocus/app/util"
	"hyperfocus/app/util/mylog"
	"os"

	"github.com/spf13/cobra"
	"go.szostok.io/version/extension"
)

func main() {
	mylog.Preinit()

	fmt.Fprintln(os.Stderr, util.Banner)

	rootCmd := &cobra.Command{Use: "hyperfocus"}
	rootCmd.AddCommand(cmd.Server)
	rootCmd.AddCommand(extension.NewVersionCobraCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
		return
	}
}
