package main

import (
	"TaoKanOperator/TaoKan/k8s"
	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	log.Infoln("Starting TaoKan ...")

	k8s := KubernetesAPI.GetInstance()

	pods, err := k8s.ListPods("hub")
	if err != nil {
		panic(err.Error())
	}

	for i, pod := range pods.Items {
		log.Infof("%d: %v", i, pod.Name)
	}
}
