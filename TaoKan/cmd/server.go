package cmd

import (
	"TaoKanOperator/TaoKan/k8s"
	"fmt"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"strings"
)

var serverPort int32

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
	serverCmd.Flags().Int32VarP(&serverPort, "port", "p", 2222, "Server port to listen on")
}

func serverStatus(s ssh.Session) {
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	pods, err := k8s.ListPods(Namespace)
	if err != nil {
		io.WriteString(s, "[Error] "+err.Error()+"\n")
		return
	}

	result := ""
	for _, pod := range pods {
		result += fmt.Sprintf("%s\n", pod.Name)
	}
	io.WriteString(s, result)
}

func mountPvcByRsyncServer(s ssh.Session, pvcName string) error {
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	_, usedByPods, err := k8s.GetPvc(Namespace, pvcName)
	if err != nil {
		return err
	}

	result := ""
	isRsyncServerRunning := false
	if len(usedByPods) > 0 {
		var pods []string
		for _, pod := range usedByPods {
			if val, ok := pod.Labels["managed-by"]; ok && val == "TaoKan" {
				isRsyncServerRunning = true
			}
			pods = append(pods, pod.Name)
		}
		log.Warnf("[Used By] Pod " + strings.Join(pods, ","))
		result += "Mounted by pod: " + strings.Join(pods, ",") + "\n"
	}
	io.WriteString(s, result)
	if isRsyncServerRunning == false {
		// Launch rsync-server
		log.Infoln("[Launch] rsync-worker to mount pvc " + pvcName)
		err := k8s.LaunchRsyncServerPod(Namespace, pvcName)
		if err != nil {
			return err
		}
	} else {
		log.Warnf("[SKip] pod rsync-worker" + pvcName + " is already running\n")
	}

	return nil
}

func unmountPvcFromRsyncServer(s ssh.Session, pvcName string) error {
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	pvc, usedByPods, err := k8s.GetPvc(Namespace, pvcName)
	if err != nil {
		return err
	}

	isRsyncServerRunning := false
	var rsyncServerPodName string
	if len(usedByPods) > 0 {
		var pods []string
		for _, pod := range usedByPods {
			if val, ok := pod.Labels["managed-by"]; ok && val == "TaoKan" {
				isRsyncServerRunning = true
				rsyncServerPodName = pod.Name
			}
			pods = append(pods, pod.Name)
		}

		if !isRsyncServerRunning {
			log.Warnf("[Skip] Pvc is used by pods " + strings.Join(pods, ","))
			return nil
		}
	} else {
		log.Warnf("[Skip] Pvc %s is not mounted by any pods", pvcName)
		return nil
	}

	if pvc == nil {
		log.Warnf("[Skip] Pvc %s not found", pvcName)
		return nil
	}

	if isRsyncServerRunning {
		log.Infof("[Delete] Pod %s\n", rsyncServerPodName)
		err = k8s.DeletePod(Namespace, rsyncServerPodName)
		if err != nil {
			return err
		}
	}
	return nil
}

func initSSHServer(port int32) {
	ssh.Handle(func(s ssh.Session) {
		const welcomeMsg = "[TaoKan Server]\n"
		io.WriteString(s, welcomeMsg)

		receivedCommands := s.Command()

		if len(receivedCommands) == 0 {
			io.WriteString(s, "[Error] No command provided.\n")
			s.Close()
			return
		}

		switch receivedCommands[0] {
		case "status":
			io.WriteString(s, "TaoKan Server Status\n")
			serverStatus(s)
		case "list", "ls":
			io.WriteString(s, "list\n")
		case "mount":
			//	Mount the provided pvc by rsync-server pod
			if len(receivedCommands) < 2 {
				io.WriteString(s, "[Error] Should provide PVC.\n")
				return
			}
			pvc := receivedCommands[1]
			io.WriteString(s, "Mount pvc "+pvc+"\n")
			log.Infoln("[Mount] pvc: " + pvc)
			err := mountPvcByRsyncServer(s, pvc)
			if err != nil {
				io.WriteString(s, "[Error] "+err.Error()+"\n")
				log.Error(err)
			}
		case "umount", "unmount":
			if len(receivedCommands) < 2 {
				io.WriteString(s, "[Error] Should provide PVC.\n")
				return
			}
			pvc := receivedCommands[1]
			io.WriteString(s, "Unmount pvc "+pvc+"\n")
			log.Infoln("[Unmount] pvc: " + pvc)
			err := unmountPvcFromRsyncServer(s, pvc)
			if err != nil {
				io.WriteString(s, "[Error] "+err.Error()+"\n")
				log.Error(err)
			}
		default:
			io.WriteString(s, "Unsupported command '"+receivedCommands[0]+"'\n")
		}
	})
	addr := fmt.Sprintf(":%d", port)
	go log.Fatal(ssh.ListenAndServe(addr, nil))
}

func serverEntrypoint(cmd *cobra.Command, args []string) {

	log.Infoln("kubeconfig:", KubeConfig)
	log.Infoln("namespace:", Namespace)

	log.Infof("Start ssh server at %d\n", serverPort)
	initSSHServer(serverPort)
}
