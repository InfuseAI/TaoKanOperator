package cmd

import (
	"TaoKanOperator/TaoKan/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Send the pvc data to remote cluster",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		log.Infoln("Start TaoKan client mode")
		log.Infoln("kubeconfig:", KubeConfig)
		log.Infoln("namespace:", Namespace)
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
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clientCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// clientCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
