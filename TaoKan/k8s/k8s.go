package KubernetesAPI

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
	"sync"
	"time"
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
			log.Infoln("Init k8s instance")
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

func (k *KubernetesCluster) DeletePod(namespace string, podName string) error {
	ctx := context.TODO()
	err := k.Clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	timeoutSeconds := int64(60)
	watcher, err := k.Clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + podName,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		return err
	}

	for event := range watcher.ResultChan() {
		if event.Type == watch.Deleted {
			log.Infof("[Deleted] Pod %s\n", podName)
			break
		}
	}

	return nil
}

func (k *KubernetesCluster) GetPvc(namespace string, pvcName string) (*v1.PersistentVolumeClaim, []v1.Pod, error) {
	pvc, err := k.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	usedPods, err := k.ListPodsUsePvc(namespace, pvcName)
	if err != nil {
		return pvc, nil, err
	}
	return pvc, usedPods, err
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

//go:embed rsync-server.yaml
var RsyncServerYamlTemplate []byte

func (k *KubernetesCluster) LaunchRsyncServerPod(namespace string, pvcName string) error {
	var podTemplate v1.Pod
	err := yaml.Unmarshal(RsyncServerYamlTemplate, &podTemplate)
	if err != nil {
		return err
	}

	// Prepare the pod template
	podTemplate.Name = fmt.Sprintf("rsync-server-%s", pvcName)
	podTemplate.Namespace = namespace
	podTemplate.Labels["mountPvc"] = pvcName
	podTemplate.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = pvcName

	// Apply pod
	_, err = k.Clientset.CoreV1().Pods(namespace).Create(context.TODO(), &podTemplate, metav1.CreateOptions{})

	// Check Service
	retryTimes := 3
	retryDuration := 5 * time.Second
	var svc *v1.Service
	for i := 1; i <= retryTimes; i++ {
		svc, err = k.Clientset.CoreV1().Services(namespace).Get(context.TODO(), podTemplate.Name, metav1.GetOptions{})
		if err != nil {
			log.Warnf("Get service %s failed: %v, retry #%d ...\n", podTemplate.Name, err, i)
			time.Sleep(retryDuration)
			continue
		}
		break
	}
	if err != nil {
		return err
	}
	log.Infof("Service %s found\n", svc.Name)

	// Wait until rsync-server pod ready
	selector := "metadata.name=" + podTemplate.Name
	timeoutSeconds := int64(60 * 5)
	ctx := context.TODO()
	watcher, err := k.Clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:  selector,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		return err
	}

	isRsyncServerReady := false
	for event := range watcher.ResultChan() {
		if event.Object == nil {
			break
		}
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			break
		}
		containerState := pod.Status.ContainerStatuses[0].State
		var msg string
		if pod.Status.ContainerStatuses[0].State.Waiting != nil {
			msg = containerState.Waiting.Reason
		} else if containerState.Running != nil {
			msg = "Running"
		} else if containerState.Terminated != nil {
			msg = containerState.Terminated.Reason
		}

		log.Infof("Pod: %s Status: %v\n", pod.Name, msg)
		if pod.Status.Phase == v1.PodPending {
			continue
		} else if pod.Status.Phase == v1.PodRunning {
			log.Infof("[Ready] %s\n", pod.Name)
			isRsyncServerReady = true
			break
		} else {
			break
		}
	}

	if !isRsyncServerReady {
		err = errors.New(fmt.Sprintf("pod %s is not ready", podTemplate.Name))
	}

	return err
}
