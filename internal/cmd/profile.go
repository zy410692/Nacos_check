package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"nacos-check/internal/core"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "查看版本",
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version: 0.7.3")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "查看本地配置文件路径",
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		configfile := core.GetConfigFilePath()
		fmt.Println("本地配置文件路径:", configfile)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}
