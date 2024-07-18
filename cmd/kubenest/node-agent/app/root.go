package app

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app/client"
	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app/logger"
	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app/serve"
)

var (
	user     string // username for authentication
	password string // password for authentication
	log      = logger.GetLogger()
)

var RootCmd = &cobra.Command{
	Use:   "node-agent",
	Short: "node-agent is a tool for node to start websocket server and client",
	Long: `node-agent client for connect server to execute command and upload file to node
            node-agent serve for start websocket server to receive message from client and download file from client`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func initConfig() {
	// Tell Viper to automatically look for a .env file
	viper.SetConfigFile("agent.env")
	currentDir, _ := os.Getwd()
	viper.AddConfigPath(currentDir)
	viper.AddConfigPath("/srv/node-agent/agent.env")
	// If a agent.env file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.Warnf("Load config file error, %s", err)
	}
	// set default value from agent.env
	if len(user) == 0 {
		user = viper.GetString("WEB_USER")
	}
	if len(password) == 0 {
		password = viper.GetString("WEB_PASS")
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVarP(&user, "user", "u", "", "Username for authentication")
	RootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "Password for authentication")
	// bind flags to viper
	err := viper.BindPFlag("WEB_USER", RootCmd.PersistentFlags().Lookup("user"))
	if err != nil {
		log.Fatal(err)
	}
	err = viper.BindPFlag("WEB_PASS", RootCmd.PersistentFlags().Lookup("password"))
	if err != nil {
		log.Fatal(err)
	}
	// bind environment variables
	err = viper.BindEnv("WEB_USER", "WEB_USER")
	if err != nil {
		log.Fatal(err)
	}
	err = viper.BindEnv("WEB_PASS", "WEB_PASS")
	if err != nil {
		log.Fatal(err)
	}
	RootCmd.AddCommand(client.ClientCmd)
	RootCmd.AddCommand(serve.ServeCmd)
}
