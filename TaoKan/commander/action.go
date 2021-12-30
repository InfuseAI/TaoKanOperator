package commander

import (
	KubernetesAPI "TaoKan/k8s"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	v1 "k8s.io/api/core/v1"
	"strings"
)

func getRsyncServerStatus(pvcName string) (string, error) {
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	_, usedByPods, err := k8s.GetPvc(Namespace, pvcName)
	if err != nil {
		return "", err
	}

	rsyncServer := ""
	if len(usedByPods) > 0 {
		var pods []string
		for _, pod := range usedByPods {
			managedBy := pod.Labels["managed-by"]
			role := pod.Labels["role"]
			if managedBy == "TaoKan" && role == "rsync-server" {
				rsyncServer = pod.Name
			}
			pods = append(pods, pod.Name)
		}
		log.Warnf("[Used By] Pod " + strings.Join(pods, ","))
	}
	return rsyncServer, nil
}

func status(w io.Writer, args []string) error {
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	var result string

	log.Infof("List User PVC ...")
	userPvcs, err := k8s.ListUserPvc(Namespace)
	if err != nil {
		return err
	}
	io.WriteString(w, "[User] PVC\n")
	result, err = k8s.ShowPvcStatus(Namespace, userPvcs)
	if err != nil {
		return err
	}
	io.WriteString(w, result)
	log.Infof("Found %d PVCs", len(userPvcs))

	log.Infof("List Dataset PVC ...")
	datasetPvcs, err := k8s.ListDatasetPvc(Namespace)
	if err != nil {
		return err
	}
	io.WriteString(w, "[Dataset] PVC\n")
	result, err = k8s.ShowPvcStatus(Namespace, datasetPvcs)
	if err != nil {
		return err
	}
	io.WriteString(w, result)
	log.Infof("Found %d PVCs", len(datasetPvcs))

	log.Infof("List Project PVC ...")
	projectPvcs, err := k8s.ListProjectPvc(Namespace)
	if err != nil {
		return err
	}
	io.WriteString(w, "[Project] PVC\n")
	result, err = k8s.ShowPvcStatus(Namespace, projectPvcs)
	if err != nil {
		return err
	}
	io.WriteString(w, result)
	log.Infof("Found %d PVCs", len(projectPvcs))

	return nil
}

func mountPvc(w io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("should provide PVC")
	}
	pvcName := args[0]
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	result := ""
	serverPod, err := getRsyncServerStatus(pvcName)
	if err != nil {
		return err
	}

	if serverPod == "" {
		// Launch rsync-server
		log.Infoln("[Launch] rsync-server to mount pvc " + pvcName)
		err := k8s.LaunchRsyncServerPod(Namespace, pvcName)
		if err != nil {
			return err
		}
	} else {
		log.Warnf("[Skip] pod " + serverPod + " is already running")
	}
	result = "Server pod ready: rsync-worker-" + pvcName
	io.WriteString(w, result)

	return nil
}

func umountPvc(w io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("should provide PVC")
	}
	pvcName := args[0]

	k8s := KubernetesAPI.GetInstance(KubeConfig)
	serverPod, err := getRsyncServerStatus(pvcName)
	if err != nil {
		return err
	}

	if serverPod != "" {
		log.Infof("[Delete] Pod %s", serverPod)
		go k8s.DeletePod(Namespace, serverPod)
	}
	return nil
}

func touchPvc(w io.Writer, args []string) error {
	argc := len(args)
	if argc != 3 && argc != 4 {
		return fmt.Errorf("invalid number of arguments: %d", argc)
	}
	pvcType := args[0]
	name := args[1]
	capacity := args[2]

	k8s := KubernetesAPI.GetInstance(KubeConfig)
	var err error
	switch pvcType {
	case "user":
		err = k8s.CreateUserPvc(Namespace, name, capacity)
	case "project":
		err = k8s.CreateProjectPvc(Namespace, name, capacity)
	case "dataset":
		err = k8s.CreateDatasetPvc(Namespace, name, capacity)
	case "raw":
		if argc != 4 {
			return fmt.Errorf("invalid number of arguments: %d", argc)
		}
		accessMode := args[3]
		err = k8s.CreateRawPvc(Namespace, name, capacity, v1.PersistentVolumeAccessMode(accessMode))
	default:
		err = errors.New("unsupported PVC type")
	}
	return err
}
