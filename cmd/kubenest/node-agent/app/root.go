package app

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	user     string // username for authentication
	password string // password for authentication
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

	// If a .env file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
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
	viper.BindPFlag("WEB_USER", RootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("WEB_PASS", RootCmd.PersistentFlags().Lookup("password"))
	// bind environment variables
	viper.BindEnv("WEB_USER", "WEB_USER")
	viper.BindEnv("WEB_PASS", "WEB_PASS")
	RootCmd.AddCommand(ClientCmd)
	RootCmd.AddCommand(ServeCmd)
}
