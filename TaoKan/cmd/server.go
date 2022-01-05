package cmd

import (
	"TaoKan/commander"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"strings"
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
	serverCmd.Flags().UintVarP(&serverPort, "port", "p", 2022, "Server port to listen on")
	serverCmd.Flags().String("storage-class", "", "Specify the storage class for RWO pvc")
	serverCmd.Flags().String("storage-class-rwx", "", "Specify the storage class for RWX pvc")
	serverCmd.PersistentFlags().Int32("retry", 3, "Rsync-server pod restart time")
}

func serverEntrypoint(cmd *cobra.Command, args []string) {

	log.Infoln("kubeconfig:", KubeConfig)
	log.Infoln("namespace:", Namespace)
	registry := strings.TrimRight(viper.GetString("registry"), "/")
	tag := viper.GetString("image-tag")

	log.Infof("embed image: %s/infuseai/rsync-server:%s", registry, tag)
	pullPolicy := string(v1.PullAlways)
	if string(v1.PullIfNotPresent) == viper.GetString("image-pull-policy") {
		pullPolicy = string(v1.PullIfNotPresent)
	}
	log.Infof("pull policy: %s", pullPolicy)

	rwo, _ := cmd.Flags().GetString("storage-class")
	rwx, _ := cmd.Flags().GetString("storage-class-rwx")
	if rwo != "" || rwx != "" {
		log.Infof("default storage class         : %s", rwo)
		log.Infof("default storage class for RWX : %s", rwx)
	}

	log.Infof("Start ssh server at %d", serverPort)
	config := commander.Config{
		KubeConfig:      KubeConfig,
		Namespace:       Namespace,
		Port:            serverPort,
		StorageClassRWO: rwo,
		StorageClassRWX: rwx,
	}
	commander.StartServer(config)
}
