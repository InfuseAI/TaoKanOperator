package cmd

import (
	"TaoKanOperator/TaoKan/commander"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var serverPort uint

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Infoln("Start TaoKan server mode")
		serverEntrypoint(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serverCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serverCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	serverCmd.Flags().UintVarP(&serverPort, "port", "p", 2222, "Server port to listen on")
}

func serverEntrypoint(cmd *cobra.Command, args []string) {

	log.Infoln("kubeconfig:", KubeConfig)
	log.Infoln("namespace:", Namespace)

	log.Infof("Start ssh server at %d\n", serverPort)
	config := commander.Config{
		KubeConfig: KubeConfig,
		Namespace:  Namespace,
		Port:       serverPort,
	}
	commander.StartServer(config)

}
