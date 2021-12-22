package commander

import (
	KubernetesAPI "TaoKanOperator/TaoKan/k8s"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"strings"
)

func status(w io.Writer, args []string) error {

	k8s := KubernetesAPI.GetInstance(KubeConfig)
	pods, err := k8s.ListPods(Namespace)
	if err != nil {
		return err
	}

	result := ""
	for _, pod := range pods {
		result += fmt.Sprintf("%s\n", pod.Name)
	}
	io.WriteString(w, result)
	return nil
}

func mountPvc(w io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("[Error] Should provide PVC.\n")
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
		log.Warnf("[SKip] pod rsync-worker" + pvcName + " is already running\n")
	}

	return nil
}

func umountPvc(w io.Writer, args []string) error {
	if len(args) < 1 {
		return errors.New("[Error] Should provide PVC.\n")
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
		log.Infof("[Delete] Pod %s\n", rsyncServerPodName)
		err = k8s.DeletePod(Namespace, rsyncServerPodName)
		if err != nil {
			return err
		}
	}
	return nil
}
