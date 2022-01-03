package KubernetesAPI

import (
	"context"
	_ "embed"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	watchTool "k8s.io/client-go/tools/watch"
	"os"
	"strings"
	"sync"
	"time"
)

var lock = &sync.Mutex{}

type storageClass struct {
	rwo string
	rwx string
}
type KubernetesCluster struct {
	Clientset *kubernetes.Clientset

	defaultStorageClass storageClass
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

func (k *KubernetesCluster) SetRwoStorageClass(storageClass string) {
	k.defaultStorageClass.rwo = storageClass
}

func (k *KubernetesCluster) SetRwxStorageClass(storageClass string) {
	k.defaultStorageClass.rwx = storageClass
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

func parseContainerStatus(pod *v1.Pod) (status string, reason string, message string, restartCount int32) {
	if len(pod.Status.ContainerStatuses) > 0 {
		containerState := pod.Status.ContainerStatuses[0].State
		if pod.Status.ContainerStatuses[0].State.Waiting != nil {
			reason = containerState.Waiting.Reason
			message = containerState.Waiting.Message
			status = "Waiting"
		} else if containerState.Running != nil {
			status = "Running"
		} else if containerState.Terminated != nil {
			reason = containerState.Terminated.Reason
			message = containerState.Terminated.Message
			status = "Terminated"
		}
		restartCount = pod.Status.ContainerStatuses[0].RestartCount
	} else {
		status = "Pending"
	}
	return status, reason, message, restartCount
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

	// Add registry as the prefix of image name
	registry := strings.TrimRight(viper.GetString("registry"), "/")
	imageName := strings.Split(podTemplate.Spec.Containers[0].Image, ":")[0]
	imageTag := viper.GetString("image-tag")
	podTemplate.Spec.Containers[0].Image = fmt.Sprintf("%s/%s:%s", registry, imageName, imageTag)

	// Apply pod
	pod, err := k.Clientset.CoreV1().Pods(namespace).Create(context.TODO(), &podTemplate, metav1.CreateOptions{})

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
	err = k.WatchPod(*pod, v1.PodRunning, 0)
	if err != nil {
		return err
	}
	return nil

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

	// Add registry as the prefix of image name
	registry := strings.TrimRight(viper.GetString("registry"), "/")
	imageName := strings.Split(podTemplate.Spec.Containers[0].Image, ":")[0]
	imageTag := viper.GetString("image-tag")
	podTemplate.Spec.Containers[0].Image = fmt.Sprintf("%s/%s:%s", registry, imageName, imageTag)

	// Delete the existing pod
	err = k.DeletePod(namespace, podTemplate.Name)
	if err != nil {
		log.Warn(err)
	}

	// Apply pod
	pod, err := k.Clientset.CoreV1().Pods(namespace).Create(context.TODO(), &podTemplate, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	log.Infof("Pod %s created", podTemplate.Name)

	// Wait until rsync-worker pod completed
	err = k.WatchPod(*pod, v1.PodSucceeded, retryTimes)
	if err != nil {
		if pod.Spec.RestartPolicy != v1.RestartPolicyNever {
			k.DeletePod(pod.Namespace, pod.Name)
		}
		return err
	}
	return nil
}

func (k *KubernetesCluster) WatchPod(podTemplate v1.Pod, watchUntil v1.PodPhase, retryTimes int32) error {
	ctx := context.TODO()
	selector := "metadata.name=" + podTemplate.Name
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		timeout := int64(5 * 60)
		return k.Clientset.CoreV1().Pods(podTemplate.Namespace).Watch(ctx, metav1.ListOptions{
			FieldSelector:  selector,
			TimeoutSeconds: &timeout,
		})
	}
	watcher, err := watchTool.NewRetryWatcher("1", &cache.ListWatch{
		WatchFunc: watchFunc,
	})
	defer watcher.Stop()
	if err != nil {
		return err
	}

	for {
		select {
		case e, ok := <-watcher.ResultChan():
			if !ok {
				continue
			}
			pod, ok := e.Object.(*v1.Pod)
			if !ok {
				continue
			}
			phase := pod.Status.Phase
			status, reason, msg, restartCount := parseContainerStatus(pod)
			log.Debugf("Pod: %s Phase: %v status: %v:%v", pod.Name, pod.Status.Phase, status, reason)
			switch phase {
			case v1.PodPending:
				if msg != "" {
					return fmt.Errorf("[%v] Pod: %s reason: %v msg: %s", status, pod.Name, reason, msg)
				}
				continue
			case v1.PodRunning:
				switch status {
				case "Terminated":
					if restartCount > 0 {
						log.Errorf("[%v] Pod: %s reason: %v retry: %d/%d", status, pod.Name, reason, restartCount, retryTimes)
					}
				case "Waiting":
					log.Errorf("[%v] Pod: %s reason: %v message: %v", status, pod.Name, reason, msg)
				case "Running":
					log.Infof("[Running] Pod: %s", pod.Name)
					if watchUntil == v1.PodRunning {
						return nil
					}
				}
			case v1.PodSucceeded:
				log.Infof("[Completed] Pod: %s", pod.Name)
				return nil
			case v1.PodFailed:
				return fmt.Errorf("[%v] Pod: %s reason: %v retry: %d/%d", status, pod.Name, reason, restartCount, retryTimes)
			default:
				return fmt.Errorf("unsupported phase: %v", phase)
			}
			if restartCount >= retryTimes {
				return fmt.Errorf("[Abort] after retry %d times", retryTimes)
			}
		}
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

//go:embed user-pvc-template.yaml
var UserPvcTemplate []byte

//go:embed volume-pvc-template.yaml
var VolumePvcTemplate []byte

func (k *KubernetesCluster) CreatePvc(pvcTemplate v1.PersistentVolumeClaim) error {

	if pvcTemplate.Spec.AccessModes[0] == v1.ReadWriteMany && k.defaultStorageClass.rwx != "" {
		log.Debugf("Set RWX stroage class: %s", k.defaultStorageClass.rwx)
		pvcTemplate.Spec.StorageClassName = &k.defaultStorageClass.rwx
	} else if k.defaultStorageClass.rwo != "" {
		log.Debugf("Set RWO stroage class: %s", k.defaultStorageClass.rwo)
		pvcTemplate.Spec.StorageClassName = &k.defaultStorageClass.rwo
	}

	pvc, err := k.Clientset.CoreV1().PersistentVolumeClaims(pvcTemplate.Namespace).Create(context.TODO(), &pvcTemplate, metav1.CreateOptions{})
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			log.Infof("[Touched] %v", err)
			return nil
		}
		return err
	}
	log.Warnf("[Created] pvc: %v accessModes: %v sc: %v", pvc.Name, pvc.Spec.AccessModes, *pvc.Spec.StorageClassName)
	return nil
}

func (k *KubernetesCluster) CreateUserPvc(namespace string, name string, capacityString string) error {
	var pvcTemplate v1.PersistentVolumeClaim
	err := yaml.Unmarshal(UserPvcTemplate, &pvcTemplate)
	if err != nil {
		return err
	}

	capacity, err := resource.ParseQuantity(capacityString)
	if err != nil {
		return err
	}

	pvcTemplate.Annotations["hub.jupyter.org/username"] = name
	pvcTemplate.Name = fmt.Sprintf("claim-%s", name)
	pvcTemplate.Namespace = namespace
	pvcTemplate.Spec.Resources.Requests["storage"] = capacity

	return k.CreatePvc(pvcTemplate)
}

func (k *KubernetesCluster) CreateProjectPvc(namespace string, name string, capacityString string) error {
	var pvcTemplate v1.PersistentVolumeClaim
	err := yaml.Unmarshal(VolumePvcTemplate, &pvcTemplate)
	if err != nil {
		return err
	}
	capacity, err := resource.ParseQuantity(capacityString)
	if err != nil {
		return err
	}
	pvcTemplate.Labels["primehub-group"] = name
	pvcTemplate.Name = fmt.Sprintf("data-nfs-project-%s-0", name)
	pvcTemplate.Namespace = namespace
	pvcTemplate.Spec.Resources.Requests["storage"] = capacity

	return k.CreatePvc(pvcTemplate)
}

func (k *KubernetesCluster) CreateRawPvc(namespace string, name string, capacityString string, accessMode v1.PersistentVolumeAccessMode) error {
	var pvcTemplate v1.PersistentVolumeClaim
	capacity, err := resource.ParseQuantity(capacityString)
	if err != nil {
		return err
	}
	pvcTemplate.Name = name
	pvcTemplate.Namespace = namespace
	pvcTemplate.Spec.Resources.Requests = v1.ResourceList{"storage": capacity}
	pvcTemplate.Spec.AccessModes = []v1.PersistentVolumeAccessMode{accessMode}

	return k.CreatePvc(pvcTemplate)
}

func (k *KubernetesCluster) CreateDatasetPvc(namespace string, name string, capacityString string) error {
	var pvcTemplate v1.PersistentVolumeClaim
	err := yaml.Unmarshal(VolumePvcTemplate, &pvcTemplate)
	if err != nil {
		return err
	}
	capacity, err := resource.ParseQuantity(capacityString)
	if err != nil {
		return err
	}
	pvcTemplate.Labels["primehub-group"] = fmt.Sprintf("dataset-%s", name)
	pvcTemplate.Name = fmt.Sprintf("data-nfs-dataset-%s-0", name)
	pvcTemplate.Namespace = namespace
	pvcTemplate.Spec.Resources.Requests["storage"] = capacity

	return k.CreatePvc(pvcTemplate)
}
