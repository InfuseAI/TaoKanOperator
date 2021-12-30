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

		log.Infoln("Start TaoKan to transfer data to remote cluster by rsync")
		pvcName := args[0]
		k8s := KubernetesAPI.GetInstance(KubeConfig)

		pvc, usedByPods, err := k8s.GetPvc(Namespace, pvcName)
		if err != nil {
			log.Fatal(err)
		}
		if len(usedByPods) > 0 {
			log.Warnf("[Warning] Pvc %s is used by Pod %s", pvcName, usedByPods[0].Name)
		}

		transferPvcData(cmd, []v1.PersistentVolumeClaim{*pvc})
	},
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup <pvc-name>",
	Short: "Cleanup the existing rsync worker pod",
	Long:  ``,
	Args:  cobra.RangeArgs(1, 1),
	Run: func(cmd *cobra.Command, args []string) {
		pvcName := args[0]
		if pvcName == "ALL" {
			log.Infof("Start cleanup all the rsync worker & rsync server pods")
			k8s := KubernetesAPI.GetInstance(KubeConfig)
			workerPods, err := k8s.ListPodsByFilter(Namespace, func(pod v1.Pod) bool {
				if strings.HasPrefix(pod.Name, "rsync-worker") {
					return true
				}
				return false
			})
			serverPods, err := k8s.ListPodsByFilter(Namespace, func(pod v1.Pod) bool {
				if strings.HasPrefix(pod.Name, "rsync-server") {
					return true
				}
				return false
			})
			if err != nil {
				log.WithError(err)
				return
			}
			for _, pod := range workerPods {
				k8s.DeletePod(Namespace, pod.Name)
			}
			for _, pod := range serverPods {
				k8s.DeletePod(Namespace, pod.Name)
			}
		} else {
			log.Infoln("Start cleanup the rsync worker pods related with pvc " + pvcName)
			k8s := KubernetesAPI.GetInstance(KubeConfig)
			pods, err := k8s.ListPodsUsePvc(Namespace, pvcName)
			if err != nil {
				log.Warn(err)
				return
			}
			rsyncWorkerPodName := fmt.Sprintf("rsync-worker-%s", pvcName)

			isRsyncWorkerFound := false
			for _, pod := range pods {
				if rsyncWorkerPodName == pod.Name {
					log.Infof("[Delete] pod %v", pod.Name)
					err = k8s.DeletePod(Namespace, pod.Name)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
			if !isRsyncWorkerFound {
				log.Warnf("[Skip] Pod %v not found", rsyncWorkerPodName)
			}
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

	clientCmd.Flags().Bool("daemon", false, "Enable daemon mode")

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
	if daemonMode, _ := cmd.Flags().GetBool("daemon"); daemonMode {
		go transferBackupData(cmd, backupList)

		// Wait forever
		for {
			c := make(chan int)
			<-c
		}
	} else {
		transferBackupData(cmd, backupList)
	}
}

func transferBackupData(cmd *cobra.Command, backupList backupList) {
	log.Infof("Process User data transfer")
	transferPvcData(cmd, backupList.userPvcs)

	log.Infof("Process Project data transfer")
	transferPvcData(cmd, backupList.projectPvcs)

	log.Infof("Process Dataset data transfer")
	transferPvcData(cmd, backupList.datasetPvcs)
	log.Infof("[Completed] transfer backup data ")
}

func transferPvcData(cmd *cobra.Command, pvcs []v1.PersistentVolumeClaim) {
	for _, pvc := range pvcs {
		// Ask remote cluster to touch PVC by rsyncServer pod
		err := touchRemotePvc(cmd, pvc)
		if err != nil {
			log.Warnf("[Skip] pvc %s : %v", pvc.Name, err)
			continue
		}

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

		if !isRsyncServerReady {
			log.Warnf("[Skip] Pvc %s due to rsync-server not running", pvc.Name)
			continue
		}

		k8s := KubernetesAPI.GetInstance(KubeConfig)
		retryTimes, _ := cmd.Flags().GetInt32("retry")
		err = k8s.LaunchRsyncWorkerPod(RemoteCluster, Namespace, pvc.Name, retryTimes)
		if err != nil {
			log.Errorf("[Failed] Launch worker %v :%v", "rsync-worker-"+pvc.Name, err)
		}

		outputLogs, err = commanderWrapper(cmd, "umount", pvc.Name)
		if err != nil {
			log.Warnf("[Skip] Umount pvc %s from server : %v", pvc.Name, err)
			continue
		}
		for _, d := range outputLogs {
			if d != "" {
				log.Infof(d)
			}
		}
	}
}

func touchRemotePvc(cmd *cobra.Command, pvc v1.PersistentVolumeClaim) error {
	var pvcType string
	var name string
	var capacity string
	var accessMode string

	if userName, ok := pvc.Annotations["hub.jupyter.org/username"]; ok {
		pvcType = "user"
		name = userName
	} else if volumeName, ok := pvc.Labels["primehub-group"]; ok {
		if strings.HasPrefix(volumeName, "dataset-") {
			pvcType = "dataset"
			name = volumeName[len("dataset-"):]
		} else {
			pvcType = "project"
			name = volumeName
		}
	} else {
		pvcType = "raw"
		name = pvc.Name
		accessMode = string(pvc.Spec.AccessModes[0])
	}
	capacity = pvc.Spec.Resources.Requests.Storage().String()

	outputLogs, err := commanderWrapper(cmd, "touch", pvcType, name, capacity, accessMode)
	if err != nil {
		return err
	}
	for _, d := range outputLogs {
		if d != "" {
			log.Infof(d)
		}
	}
	return nil
}

func showAvaliblePvcs(namespace string) {
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

	log.Infof("[User] backup list")
	for _, pvc := range pvcs {
		log.Infof("\t%s", pvc.Name)
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
	log.Infof("[Porject] backup list")
	for _, pvc := range pvcs {
		log.Infof("\t%s", pvc.Name)
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

	log.Infof("[Dataset] backup list")
	for _, pvc := range pvcs {
		log.Infof("\t%s", pvc.Name)
	}
	backupList.datasetPvcs = pvcs

	return
}

func exclusiveListFactory(cmd *cobra.Command, pvcs []v1.PersistentVolumeClaim, flagName string) ([]v1.PersistentVolumeClaim, error) {
	path, err := cmd.Flags().GetString(flagName)
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
		log.Debugf("[Skip] %s: %v", flagName, err)
	} else {
		log.Debugf("[Load] %s from path: %s", flagName, path)
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
	path, err := cmd.Flags().GetString(flagName)
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

	if len(whiteList) == 0 {
		err = errors.New(fmt.Sprintf("White list %v is empty", flagName))
	}
	if err != nil {
		log.Debugf("[Skip] %s: %v", flagName, err)
		pvcs, _ = listFunc(namespace)
	} else {
		log.Debugf("[Load] %s from path: %s", flagName, path)
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
