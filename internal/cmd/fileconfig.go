package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"io/ioutil"
	"nacos-check/internal/config"
	"nacos-check/pkg"
	"os"
	"path/filepath"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "查看版本",
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version: 0.6.2")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "查看本地配置文件路径",
	Run: func(cmd *cobra.Command, args []string) {
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		configfile := GetConfigFilePath()
		fmt.Println("本地配置文件路径:", configfile)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

func GetConfigFilePath() string {
	homedir, err := pkg.HomeDir()
	if err != nil {
		fmt.Println("获取系统家目录获取异常")
		homedir = "."
	}
	configfile := filepath.Join(homedir, ".nacos_conf.toml")
	return configfile
}

func NacosFilePathLoad() {
	type NewConfig struct {
		Url               string   `toml:"url"`
		Namespace         []string `toml:"namespace"`
		Group             []string `toml:"group"`
		Container_network []string `toml:"container_network"`
		Label             []map[string]string
		Ipfile            string `toml:"ipfile"`
	}
	homedir, err := pkg.HomeDir()
	if err != nil {
		homedir = "."
	}
	configfile := filepath.Join(homedir, ".nacos_conf.toml")
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("配置文件错误格式错误", configfile, err)
			os.Exit(2)
		}
	}()
	if _, err := os.Stat(configfile); err != nil {
		if !os.IsExist(err) {
			return
		}
	} else {
		var newConfig NewConfig
		_, err := toml.DecodeFile(configfile, &newConfig)
		for _, label := range newConfig.Label {
			config.ADDLABEL[label["name"]] = label["value"]
		}
		if err != nil {
			fmt.Println("配置文件错误格式错误", configfile)
			return
		}
		config.IPFILE = newConfig.Ipfile
		if config.NACOSURL == "http://dev-k8s-nacos:8848" {
			config.NACOSURL = newConfig.Url
		}
		if len(newConfig.Container_network) != 0 {
			pkg.MaxCidrBlocks = newConfig.Container_network
		}
		for _, namespace := range newConfig.Namespace {
			config.NAMESPACELIST = append(config.NAMESPACELIST, config.NamespaceServer{
				Namespace:         namespace,
				NamespaceShowName: namespace,
				Quota:             200,
				Type:              2,
			})
		}
		for _, group := range newConfig.Group {
			if !pkg.InString(group, config.GROUPLIST) {
				config.GROUPLIST = append(config.GROUPLIST, group)
			}
		}
		IPFilePathLoad()
		pkg.ContainerdInit()
	}
}

func IPFilePathLoad() {
	if _, err := os.Stat(config.IPFILE); err != nil {
		if !os.IsExist(err) {
			config.PARSEIP = false
			return
		}
	} else {
		config.PARSEIP = true
		file, err := os.OpenFile(config.IPFILE, os.O_RDONLY, 0644)
		if err != nil {
			fmt.Println("打开文件错误")
			os.Exit(config.EXITCODE)
		}
		defer file.Close()
		bytefile, _ := ioutil.ReadAll(file)
		if err := json.Unmarshal(bytefile, &config.IPDATA); err != nil {
			fmt.Println("ip文件解析错误,请确认json格式")
			config.PARSEIP = false
		}
	}
}