package KubernetesAPI

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

var lock = &sync.Mutex{}

type KubernetesCluster struct {
	Clientset *kubernetes.Clientset
}

var instance *KubernetesCluster

func GetInstance(kubeconfig string) *KubernetesCluster {
	if instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if instance == nil {
			log.Infoln("Create k8s instance")
			instance = &KubernetesCluster{}
			err := instance.init(kubeconfig)
			if err != nil {
				panic(err.Error())
			}
		}
	}
	return instance
}

func (k *KubernetesCluster) init(kubeconfig string) error {
	if kubeconfig != "" {
		// use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}

		// create the clientsetq
		k.Clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		return nil
	} else {
		// creates the in-cluster config
		config, err := rest.InClusterConfig()
		if err != nil {
			return err
		}
		// creates the clientset
		k.Clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *KubernetesCluster) ListPods(namespace string) ([]v1.Pod, error) {
	podList, err := k.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return podList.Items, err
}

func (k *KubernetesCluster) ListPodsByFilter(namespace string, predicate func(pod v1.Pod) bool) ([]v1.Pod, error) {
	nsPods, err := k.ListPods(namespace)
	if err != nil {
		return nil, err
	}
	var pods []v1.Pod

	for _, pod := range nsPods {
		if predicate(pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (k *KubernetesCluster) ListPodsUsePvc(namespace string, pvcName string) ([]v1.Pod, error) {
	return k.ListPodsByFilter(namespace, func(pod v1.Pod) bool {
		for _, volume := range pod.Spec.Volumes {
			if volume.VolumeSource.PersistentVolumeClaim != nil && volume.VolumeSource.PersistentVolumeClaim.ClaimName == pvcName {
				return true
			}
		}
		return false
	})
}

func (k *KubernetesCluster) ListPvc(namespace string) ([]v1.PersistentVolumeClaim, error) {
	pvcList, err := k.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pvcList.Items, nil
}

func (k *KubernetesCluster) ListPvcByFilter(namespace string, predicate func(pvc v1.PersistentVolumeClaim) bool) ([]v1.PersistentVolumeClaim, error) {
	pvcs, err := k.ListPvc(namespace)
	if err != nil {
		return nil, err
	}

	results := make([]v1.PersistentVolumeClaim, 0)
	for _, pvc := range pvcs {
		if predicate(pvc) {
			results = append(results, pvc)
		}
	}
	return results, nil
}

func (k *KubernetesCluster) ListUserPvc(namespace string) ([]v1.PersistentVolumeClaim, error) {
	return k.ListPvcByFilter(namespace, func(pvc v1.PersistentVolumeClaim) bool {
		return strings.HasPrefix(pvc.Name, "claim-")
	})
}

func (k *KubernetesCluster) ListProjectPvc(namespace string) ([]v1.PersistentVolumeClaim, error) {
	return k.ListPvcByFilter(namespace, func(pvc v1.PersistentVolumeClaim) bool {
		return strings.HasPrefix(pvc.Name, "data-nfs-project")
	})
}

func (k *KubernetesCluster) ListDatasetPvc(namespace string) ([]v1.PersistentVolumeClaim, error) {
	return k.ListPvcByFilter(namespace, func(pvc v1.PersistentVolumeClaim) bool {
		return strings.HasPrefix(pvc.Name, "data-nfs-dataset")
	})
}
