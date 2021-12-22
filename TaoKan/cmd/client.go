package cmd

import (
	"TaoKanOperator/TaoKan/commander"
	"TaoKanOperator/TaoKan/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var RemoteCluster string
var RemotePort uint

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Send the pvc data to remote cluster",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		log.Infoln("Start TaoKan client mode")
		clientEntrypoint(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clientCmd.PersistentFlags().String("foo", "", "A help for foo")
	clientCmd.PersistentFlags().StringVarP(&RemoteCluster, "remote", "r", "", "Remote cluster domain")
	clientCmd.PersistentFlags().UintVarP(&RemotePort, "port", "p", 2222, "Remote cluster port")
	clientCmd.MarkPersistentFlagRequired("remote")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// clientCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func clientEntrypoint(cmd *cobra.Command, args []string) {
	// Flow

	// Init k8s cluster
	log.Infoln("kubeconfig:", KubeConfig)
	log.Infoln("namespace:", Namespace)
	log.Infoln("remote cluster:", RemoteCluster)
	log.Infoln("retmoe port:", RemotePort)

	// Prepare selected pvc list
	//		Project & Dataset
	//  	User
	prepareSelectedPvcs(Namespace)

	// Build the connection with Server
	log.Infoln("Connecting to server ...")
	config := commander.Config{
		Namespace:  Namespace,
		KubeConfig: KubeConfig,
		Remote:     RemoteCluster,
		Port:       RemotePort,
	}
	c, err := commander.StartClient(config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		log.Infoln("Closed ssh connection")
		c.Close()
	}()
	log.Infoln("Run cmd status")
	out, err := c.Run("status")
	if err != nil {
		log.Fatal(err)
	}
	log.Infoln(string(out))
	// Transfer data processes
}

func prepareSelectedPvcs(namespace string) {
	// TODO: Load the pvc transfer list from file
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	userPvcs, err := k8s.ListUserPvc(Namespace)
	if err != nil {
		log.WithError(err)
		os.Exit(1)
	}
	for _, pvc := range userPvcs {
		log.Infoln("Pvc", pvc.Name)
		usedBy, err := k8s.ListPodsUsePvc(Namespace, pvc.Name)
		if err != nil {
			log.WithError(err)
			continue
		}
		for _, pod := range usedBy {
			log.Infoln("\tUsed by Pods:", pod.Name)
		}
	}

	datasetPvcs, err := k8s.ListDatasetPvc(Namespace)
	if err != nil {
		log.WithError(err)
		os.Exit(1)
	}
	for _, pvc := range datasetPvcs {
		log.Infoln("Pvc", pvc.Name)
		usedBy, err := k8s.ListPodsUsePvc(Namespace, pvc.Name)
		if err != nil {
			log.WithError(err)
			continue
		}
		for _, pod := range usedBy {
			log.Infoln("\tUsed by Pods:", pod.Name)
		}
	}

	projectPvcs, err := k8s.ListProjectPvc(Namespace)
	if err != nil {
		log.WithError(err)
		os.Exit(1)
	}
	for _, pvc := range projectPvcs {
		log.Infoln("Pvc", pvc.Name)
		usedBy, err := k8s.ListPodsUsePvc(Namespace, pvc.Name)
		if err != nil {
			log.WithError(err)
			continue
		}
		for _, pod := range usedBy {
			log.Infoln("\tUsed by Pods:", pod.Name)
		}
	}
}
