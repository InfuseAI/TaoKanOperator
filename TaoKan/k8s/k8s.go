package KubernetesAPI

import (
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
)

var lock = &sync.Mutex{}

type KuberneteCluster struct {
	clientset *kubernetes.Clientset
}

var instance *KuberneteCluster

func GetInstance() *KuberneteCluster {
	if instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if instance == nil {
			log.Infoln("Create k8s instance")
			instance = &KuberneteCluster{}
			err := instance.init()
			if err != nil {
				panic(err.Error())
			}
		}
	}
	return instance
}

func (k *KuberneteCluster) init() error {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return err
	}

	// create the clientset
	k.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	return nil
}

func (k *KuberneteCluster) ListPods(namespace string) (*v1.PodList, error) {
	if k.clientset == nil {
		return nil, fmt.Errorf("k8s cluster not connected")
	}
	return k.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
}
