package cmd

import (
	"TaoKan/commander"
	KubernetesAPI "TaoKan/k8s"
	"bufio"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"os"
	"strings"
)

type backupList struct {
	userPvcs    []v1.PersistentVolumeClaim
	projectPvcs []v1.PersistentVolumeClaim
	datasetPvcs []v1.PersistentVolumeClaim
}

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

		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			log.SetLevel(log.DebugLevel)
		}

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
		outputs, err := commanderWrapper(cmd, "mount", pvcName)
		for _, data := range outputs {
			log.Infof(data)
		}
		if err != nil {
			log.Error(err)
			return
		}

		// Launch Rsync worker
		retryTimes, _ := cmd.Flags().GetInt32("retry")
		err = k8s.LaunchRsyncWorkerPod(RemoteCluster, Namespace, pvcName, retryTimes)
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

const (
	userListFlag             = "user-list"
	userExclusiveListFlag    = "user-exclusive-list"
	projectListFlag          = "project-list"
	projectExclusiveListFlag = "project-exclusive-list"
	datasetListFlag          = "dataset-list"
	datasetExclusiveListFlag = "dataset-exclusive-list"
)

func init() {
	rootCmd.AddCommand(clientCmd)
	rootCmd.AddCommand(cleanupCmd)
	clientCmd.AddCommand(rsyncCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// clientCmd.PersistentFlags().String("foo", "", "A help for foo")
	clientCmd.PersistentFlags().StringVarP(&RemoteCluster, "remote", "r", "", "Remote cluster domain")
	clientCmd.PersistentFlags().UintVarP(&RemotePort, "port", "p", 2022, "Remote cluster port")
	clientCmd.MarkPersistentFlagRequired("remote")
	clientCmd.PersistentFlags().String("user-list", "", "User whitelist")
	clientCmd.PersistentFlags().String("user-exclusive-list", "", "User exclusion list")
	clientCmd.PersistentFlags().String("dataset-list", "", "Dataset whitelist")
	clientCmd.PersistentFlags().String("dataset-exclusive-list", "", "Dataset exclusion list")
	clientCmd.PersistentFlags().String("project-list", "", "Project whitelist")
	clientCmd.PersistentFlags().String("project-exclusive-list", "", "Project exclusion list")
	clientCmd.PersistentFlags().Int32("retry", 0, "Rsync-worker pod restart time")

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
	log.Infoln("[TaoKan Client]")
	//showAvaliblePvcs(Namespace)
	backupList, err := prepareBackupPvcList(cmd, Namespace)
	if err != nil {
		log.WithError(err)
	}

	// Build the connection with Server
	// Transfer data processes
	log.Infof("Process User data transfer")
	for _, pvc := range backupList.userPvcs {
		// Ask remote cluster to mount PVC by rsync-server pod
		outputLogs, err := commanderWrapper(cmd, "mount", pvc.Name)
		if err != nil {
			log.Warnf("[Skip] pvc %s : %v", pvc.Name, err)
			continue
		}
		isRsyncServerReady := false
		for _, d := range outputLogs {
			if d != "" {
				log.Infof(d)
				if strings.Contains(d, "Server pod ready:") {
					isRsyncServerReady = true
				}
			}
		}

		if isRsyncServerReady {
			k8s := KubernetesAPI.GetInstance(KubeConfig)
			retryTimes, _ := cmd.Flags().GetInt32("retry")
			err = k8s.LaunchRsyncWorkerPod(RemoteCluster, Namespace, pvc.Name, retryTimes)
			if err != nil {
				log.Warnf("Failed to launch worker %v :%v", "rsync-worker-"+pvc.Name, err)
			}
		}
	}
	log.Infof("[Completed]")
}

func showAvaliblePvcs(namespace string) {
	// TODO: Load the pvc transfer list from file
	var content string
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	userPvcs, err := k8s.ListUserPvc(Namespace)
	if err != nil {
		log.WithError(err)
		os.Exit(1)
	}
	log.Infoln("[User] PVC")
	content, err = k8s.ShowPvcStatus(Namespace, userPvcs)
	for _, data := range strings.Split(content, "\n") {
		log.Infof(data)
	}

	log.Infoln("[Dataset] PVC")
	datasetPvcs, err := k8s.ListDatasetPvc(Namespace)
	if err != nil {
		log.WithError(err)
		os.Exit(1)
	}
	content, err = k8s.ShowPvcStatus(Namespace, datasetPvcs)
	for _, data := range strings.Split(content, "\n") {
		log.Infof(data)
	}

	log.Infoln("[Project] PVC")
	projectPvcs, err := k8s.ListProjectPvc(Namespace)
	if err != nil {
		log.WithError(err)
		os.Exit(1)
	}
	content, err = k8s.ShowPvcStatus(Namespace, projectPvcs)
	for _, data := range strings.Split(content, "\n") {
		log.Infof(data)
	}
}

func openListFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	var list []string

	for scanner.Scan() {
		list = append(list, scanner.Text())
	}

	return list, nil
}

func prepareBackupPvcList(cmd *cobra.Command, namespace string) (
	backupList backupList,
	err error,
) {
	var pvcs []v1.PersistentVolumeClaim

	// Get User Backup PVC List
	pvcs, err = whiteListFactory(cmd, namespace, userListFlag)
	if err != nil {
		return
	}
	pvcs, err = exclusiveListFactory(cmd, pvcs, userExclusiveListFlag)
	if err != nil {
		return
	}
	for _, pvc := range pvcs {
		log.Infof("%s", pvc.Name)
	}
	backupList.userPvcs = pvcs

	// Get Project Backup PVC List
	pvcs, err = whiteListFactory(cmd, namespace, projectListFlag)
	if err != nil {
		return
	}
	pvcs, err = exclusiveListFactory(cmd, pvcs, projectExclusiveListFlag)
	if err != nil {
		return
	}
	for _, pvc := range pvcs {
		log.Infof("%s", pvc.Name)
	}
	backupList.projectPvcs = pvcs

	// Get Dataset Backup PVC List
	pvcs, err = whiteListFactory(cmd, namespace, datasetListFlag)
	if err != nil {
		return
	}
	pvcs, err = exclusiveListFactory(cmd, pvcs, datasetExclusiveListFlag)
	if err != nil {
		return
	}
	for _, pvc := range pvcs {
		log.Infof("%s", pvc.Name)
	}
	backupList.datasetPvcs = pvcs

	return
}

func exclusiveListFactory(cmd *cobra.Command, pvcs []v1.PersistentVolumeClaim, flagName string) ([]v1.PersistentVolumeClaim, error) {
	path, err := cmd.PersistentFlags().GetString(flagName)
	if err != nil {
		return nil, err
	}
	exclusiveList, err := openListFile(path)

	var pvcPrefix string
	var pvcPostfix string
	switch flagName {
	case userExclusiveListFlag:
		pvcPrefix = KubernetesAPI.UserPvcPrefix
		pvcPostfix = ""
	case projectExclusiveListFlag:
		pvcPrefix = KubernetesAPI.ProjectPvcPrefix
		pvcPostfix = "-0"
	case datasetExclusiveListFlag:
		pvcPrefix = KubernetesAPI.DatasetPvcPrefix
		pvcPostfix = "-0"
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported flag: %v", flagName))
	}
	if err != nil {
		log.Warnf("Skip %s: %v", flagName, err)
	} else {
		log.Infof("Load %s from path: %s", flagName, path)
		for _, name := range exclusiveList {
			for i := 0; i < len(pvcs); i++ {
				pvc := pvcs[i]
				if pvc.Name == name || pvc.Name == pvcPrefix+name+pvcPostfix {
					pvcs = append(pvcs[:i], pvcs[i+1:]...)
					i--
				}
			}
		}
	}
	return pvcs, nil
}

func whiteListFactory(cmd *cobra.Command, namespace string, flagName string) ([]v1.PersistentVolumeClaim, error) {
	var pvcs []v1.PersistentVolumeClaim
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	path, err := cmd.PersistentFlags().GetString(flagName)
	if err != nil {
		return nil, err
	}
	whiteList, err := openListFile(path)

	var listFunc func(string) ([]v1.PersistentVolumeClaim, error)
	var pvcPrefix string
	var pvcPostfix string
	switch flagName {
	case userListFlag:
		listFunc = k8s.ListUserPvc
		pvcPrefix = KubernetesAPI.UserPvcPrefix
		pvcPostfix = ""
	case projectListFlag:
		listFunc = k8s.ListProjectPvc
		pvcPrefix = KubernetesAPI.ProjectPvcPrefix
		pvcPostfix = "-0"
	case datasetListFlag:
		listFunc = k8s.ListDatasetPvc
		pvcPrefix = KubernetesAPI.DatasetPvcPrefix
		pvcPostfix = "-0"
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported flag: %v", flagName))
	}

	if err != nil {
		log.Warnf("Skip %s: %v", flagName, err)
		pvcs, _ = listFunc(namespace)
	} else {
		log.Infof("Load %s from path: %s", flagName, path)
		pvcs, _ = k8s.ListPvcByFilter(namespace, func(pvc v1.PersistentVolumeClaim) bool {
			for _, name := range whiteList {
				if pvc.Name == name || pvc.Name == pvcPrefix+name+pvcPostfix {
					return true
				}
			}
			return false
		})
	}
	return pvcs, nil
}

func commanderWrapper(cmd *cobra.Command, action string, args ...string) ([]string, error) {
	namespace, _ := cmd.Flags().GetString("namespace")
	kubeConfig, _ := cmd.Flags().GetString("kubeconfig")
	remote, _ := cmd.Flags().GetString("remote")
	port, _ := cmd.Flags().GetUint("port")

	log.Debugf("Connecting to server %v:%d ...", remote, port)
	config := commander.Config{
		Namespace:  namespace,
		KubeConfig: kubeConfig,
		Remote:     remote,
		Port:       port,
	}

	c, err := commander.StartClient(config)
	if err != nil {
		return nil, err
	}

	defer func() {
		log.Debugf("Closed ssh connection")
		c.Close()
	}()
	output, err := c.Run(action, args...)
	if err != nil {
		return nil, err
	}

	return strings.Split(output, "\n"), nil
}
