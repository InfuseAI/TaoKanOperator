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
	"os"
	"strings"
	"sync"
	"time"
)

var lock = &sync.Mutex{}

type KubernetesCluster struct {
	Clientset *kubernetes.Clientset
}

const (
	UserPvcPrefix    string = "claim-"
	ProjectPvcPrefix string = "data-nfs-project-"
	DatasetPvcPrefix string = "data-nfs-dataset-"
)

var instance *KubernetesCluster

func fileExists(name string) bool {
	_, err := os.Stat(name)
	if err == nil {
		return true
	}
	return false
}

func GetInstance(kubeconfig string) *KubernetesCluster {
	if instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if instance == nil {
			log.Debugln("Init k8s instance")
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
	if kubeconfig != "" && fileExists(kubeconfig) {
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

	ch := make(chan error)
	go func() {
		timeoutSeconds := int64(60)
		watcher, err := k.Clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
			FieldSelector:  "metadata.name=" + podName,
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			ch <- err
		}
		for event := range watcher.ResultChan() {
			if event.Type == watch.Deleted {
				log.Infof("[Deleted] Pod %s", podName)
				break
			}
		}
		ch <- nil
	}()

	err := k.Clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	err = <-ch
	return err
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

func (k *KubernetesCluster) ShowPvcStatus(namespace string, pvcs []v1.PersistentVolumeClaim) (string, error) {
	var content string

	for _, pvc := range pvcs {
		content += fmt.Sprintf("\t%s\n", pvc.Name)
		pods, err := k.ListPodsUsePvc(namespace, pvc.Name)
		if err != nil {
			return "", err
		}
		if len(pods) > 0 {
			content += "\t\tUsed by: "
			for _, pod := range pods {
				content += fmt.Sprintf("%s ", pod.Name)
			}
			content += "\n"
		}
	}
	return content, nil
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
			log.Warnf("Get service %s failed: %v, retry #%d ...", podTemplate.Name, err, i)
			time.Sleep(retryDuration)
			continue
		}
		break
	}
	if err != nil {
		return err
	}
	log.Infof("Service %s found", svc.Name)

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

		log.Infof("Pod: %s Status: %v", pod.Name, msg)
		if pod.Status.Phase == v1.PodPending {
			continue
		} else if pod.Status.Phase == v1.PodRunning {
			log.Infof("[Ready] %s", pod.Name)
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

//go:embed rsync-worker.yaml
var RsyncWorkerYamlTemplate []byte

func (k *KubernetesCluster) LaunchRsyncWorkerPod(remote string, namespace string, pvcName string, retryTimes int32) error {
	var podTemplate v1.Pod
	err := yaml.Unmarshal(RsyncWorkerYamlTemplate, &podTemplate)
	if err != nil {
		return err
	}

	podTemplate.Name = fmt.Sprintf("rsync-worker-%s", pvcName)
	podTemplate.Namespace = namespace
	podTemplate.Labels["mountPvc"] = pvcName
	podTemplate.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = pvcName
	for i, env := range podTemplate.Spec.Containers[0].Env {
		switch env.Name {
		case "REMOTE_K8S_CLUSTER":
			podTemplate.Spec.Containers[0].Env[i].Value = remote
		case "REMOTE_SERVER_NAME":
			podTemplate.Spec.Containers[0].Env[i].Value = fmt.Sprintf("rsync-server-%s", pvcName)
		case "REMOTE_NAMESPACE":
			podTemplate.Spec.Containers[0].Env[i].Value = namespace
		}
	}
	if retryTimes == 0 {
		podTemplate.Spec.RestartPolicy = v1.RestartPolicyNever
	}

	// Delete the existing pod
	err = k.DeletePod(namespace, podTemplate.Name)
	if err != nil {
		log.Warn(err)
	}

	// Apply pod
	_, err = k.Clientset.CoreV1().Pods(namespace).Create(context.TODO(), &podTemplate, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	log.Infof("Pod %s created", podTemplate.Name)

	// Wait until rsync-worker pod ready
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

	isRsyncWorkerCompleted := false
watcherLoop:
	for event := range watcher.ResultChan() {
		if event.Object == nil {
			break watcherLoop
		}
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			break watcherLoop
		}
		var status string
		var reason string
		if len(pod.Status.ContainerStatuses) > 0 {
			containerState := pod.Status.ContainerStatuses[0].State
			if pod.Status.ContainerStatuses[0].State.Waiting != nil {
				reason = containerState.Waiting.Reason
				status = "Waiting"
			} else if containerState.Running != nil {
				status = "Running"
			} else if containerState.Terminated != nil {
				reason = containerState.Terminated.Reason
				status = "Terminated"
			}
		} else {
			status = "Pending"
		}
		log.Debugf("Pod: %s Phase: %v status: %v", pod.Name, pod.Status.Phase, status)
		switch pod.Status.Phase {
		case v1.PodPending:
			continue
		case v1.PodRunning:
			if status != "Running" {
				if status == "Terminated" && pod.Status.ContainerStatuses[0].RestartCount > 0 {
					restartCount := pod.Status.ContainerStatuses[0].RestartCount
					log.Errorf("[%v] Pod: %s reason: %v retry: %d/%d", status, pod.Name, reason, restartCount, retryTimes)
				} else if status == "Waiting" {
					msg := pod.Status.ContainerStatuses[0].State.Waiting.Message
					log.Errorf("[%v] Pod: %s reason: %v message: %v", status, pod.Name, reason, msg)
				}
				if pod.Status.ContainerStatuses[0].RestartCount >= retryTimes {
					log.Errorf("Abort after retry %d times", retryTimes)
					break watcherLoop
				}
			} else {
				log.Infof("[Running] Pod: %s", pod.Name)
			}
		case v1.PodSucceeded:
			log.Infof("[Completed] Pod: %s", pod.Name)
			isRsyncWorkerCompleted = true
			break watcherLoop
		case v1.PodFailed:
			restartCount := pod.Status.ContainerStatuses[0].RestartCount
			log.Errorf("[%v] Pod: %s reason: %v retry: %d/%d", status, pod.Name, reason, restartCount, retryTimes)
			break watcherLoop
		default:
			log.Errorf("Unsupported phase: %v", pod.Status.Phase)
		}
	}
	if !isRsyncWorkerCompleted {
		if podTemplate.Spec.RestartPolicy != v1.RestartPolicyNever {
			k.DeletePod(namespace, podTemplate.Name)
		}

		return errors.New(fmt.Sprintf("Failed to backup Pvc %s", pvcName))
	}
	return nil
}

func (k *KubernetesCluster) DeleteJob(namespace string, jobName string) error {
	ctx := context.TODO()
	err := k.Clientset.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	timeoutSeconds := int64(60)
	watcher, err := k.Clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + jobName,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		return err
	}

	for event := range watcher.ResultChan() {
		if event.Type == watch.Deleted {
			log.Infof("[Deleted] Job %s", jobName)
			break
		}
	}
	return nil
}

func (k *KubernetesCluster) CleanupJob(namespace string, jobName string) error {
	ctx := context.TODO()

	err := k.DeleteJob(namespace, jobName)
	if err != nil {
		return err
	}

	ch := make(chan error)
	go func() {
		var err error
		log.Infof("[Wait] Existing pods all deleted")
		timeoutSeconds := int64(60)
		watcher, err := k.Clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
			LabelSelector:  "job-name=" + jobName,
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			ch <- err
		}

		podCounter := 0
		for event := range watcher.ResultChan() {
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				break
			}
			if event.Type == watch.Added {
				podCounter++
			}
			if event.Type == watch.Deleted {
				log.Infof("[Cleanup] Pod %s", pod.Name)
				podCounter--
				if podCounter == 0 {
					break
				}
			}
		}
		ch <- err
	}()

	timeoutSeconds := int64(5)
	log.Infof("[Cleanup] Existing pods triggered by job %s", jobName)
	err = k.Clientset.CoreV1().Pods(namespace).DeleteCollection(ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector:  "job-name=" + jobName,
			TimeoutSeconds: &timeoutSeconds,
		})
	if err != nil {
		return err
	}

	err = <-ch
	return err
}
