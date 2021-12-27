package commander

import (
	KubernetesAPI "TaoKan/k8s"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"strings"
)

func status(w io.Writer, args []string) error {
	k8s := KubernetesAPI.GetInstance(KubeConfig)
	var result string

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

	return nil
}

func mountPvc(w io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("[Error] Should provide PVC.")
	}
	pvcName := args[0]
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

	io.WriteString(w, result)
	if isRsyncServerRunning == false {
		// Launch rsync-server
		log.Infoln("[Launch] rsync-worker to mount pvc " + pvcName)
		err := k8s.LaunchRsyncServerPod(Namespace, pvcName)
		if err != nil {
			return err
		}
	} else {
		log.Warnf("[SKip] pod rsync-worker" + pvcName + " is already running")
	}

	return nil
}

func umountPvc(w io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("[Error] Should provide PVC.")
	}
	pvcName := args[0]

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
		log.Infof("[Delete] Pod %s", rsyncServerPodName)
		err = k8s.DeletePod(Namespace, rsyncServerPodName)
		if err != nil {
			return err
		}
	}
	return nil
}
