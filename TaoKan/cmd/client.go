package cmd

import (
	"TaoKan/commander"
	KubernetesAPI "TaoKan/k8s"
	"fmt"
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

var rsyncCmd = &cobra.Command{
	Use:   "rsync <pvc-name>",
	Short: "Rsync the selected pvc to remote cluster",
	Long:  ``,
	Args:  cobra.RangeArgs(1, 1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Infoln("Start TaoKan to transfer data to remote cluster by rsync")
		pvcName := args[0]
		k8s := KubernetesAPI.GetInstance(KubeConfig)

		usedByPods, err := k8s.ListPodsUsePvc(Namespace, pvcName)
		if err != nil {
			log.Fatal(err)
		}
		if len(usedByPods) > 0 {
			log.Warnf("[Skip] Pvc %s is used by Pod %s", pvcName, usedByPods[0].Name)
		}

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

		output, err := c.Run("mount", pvcName)
		if output != "" {
			log.Infoln(output)
		}
		if err != nil {
			log.Error(err)
			return
		}

		// Launch Rsync worker
		err = k8s.LaunchRsyncWorkerJob(RemoteCluster, Namespace, pvcName)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup <pvc-name>",
	Short: "Cleanup the existing rsync worker pod",
	Long:  ``,
	Args:  cobra.RangeArgs(1, 1),
	Run: func(cmd *cobra.Command, args []string) {
		pvcName := args[0]
		log.Infoln("Start cleanup the rsync worker pods related with pvc " + pvcName)
		k8s := KubernetesAPI.GetInstance(KubeConfig)
		_, err := k8s.ListPodsUsePvc(Namespace, pvcName)
		if err != nil {
			log.Warn(err)
			return
		}
		jobName := fmt.Sprintf("rsync-worker-%s", pvcName)
		err = k8s.CleanupJob(Namespace, jobName)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
	rootCmd.AddCommand(cleanupCmd)
	clientCmd.AddCommand(rsyncCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clientCmd.PersistentFlags().String("foo", "", "A help for foo")
	clientCmd.PersistentFlags().StringVarP(&RemoteCluster, "remote", "r", "", "Remote cluster domain")
	clientCmd.PersistentFlags().UintVarP(&RemotePort, "port", "p", 2222, "Remote cluster port")
	clientCmd.MarkPersistentFlagRequired("remote")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
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
	log.Infoln(out)
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
