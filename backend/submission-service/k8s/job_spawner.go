package k8s

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	namespace = "iicpc"
)

var (
	registry       = os.Getenv("REGISTRY_ADDRESS")
	registryMirror = os.Getenv("REGISTRY_MIRROR")
)

func getClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	return kubernetes.NewForConfig(config)
}

func SpawnSandboxJob(submissionID string, zipPath string) error {
	id8 := submissionID[:8]
	imageName := fmt.Sprintf("%s/submission-%s:latest", registry, id8)

	clientset, err := getClient()
	if err != nil {
		return err
	}

	// Step 1 — Kaniko Job: build image from zip and push to registry
	log.Printf("[K8S] spawning Kaniko build job for %s", id8)
	if err := spawnKanikoJob(clientset, id8, zipPath, imageName); err != nil {
		return fmt.Errorf("kaniko job failed: %w", err)
	}

	// Step 2 — Wait for Kaniko to finish
	log.Printf("[K8S] waiting for Kaniko job to complete...")
	if err := waitForJob(clientset, "kaniko-"+id8, 10*time.Minute); err != nil {
		return fmt.Errorf("kaniko job did not complete: %w", err)
	}
	log.Printf("[K8S] image %s built and pushed", imageName)

	// Step 3 — Create sandbox Pod + Service
	log.Printf("[K8S] creating sandbox pod and service for %s", id8)
	if err := spawnSandboxPod(clientset, id8, imageName, submissionID); err != nil {
		return fmt.Errorf("sandbox pod failed: %w", err)
	}

	// Step 4 — Wait for sandbox pod to be ready
	log.Printf("[K8S] waiting for sandbox pod to be ready...")
	if err := waitForPod(clientset, "sandbox-"+id8, 3*time.Minute); err != nil {
		return fmt.Errorf("sandbox pod not ready: %w", err)
	}
	log.Printf("[K8S] sandbox pod ready")

	// Step 5 — Create bot-fleet Job
	log.Printf("[K8S] spawning bot-fleet job for %s", id8)
	if err := spawnBotFleetJob(clientset, id8, submissionID); err != nil {
		return fmt.Errorf("bot-fleet job failed: %w", err)
	}

	log.Printf("[K8S] pipeline complete for submission %s", submissionID)
	return nil
}

func spawnKanikoJob(clientset *kubernetes.Clientset, id8, zipPath, imageName string) error {
	ttl := int32(300)
	backoff := int32(0)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kaniko-" + id8,
			Namespace: namespace,
			Labels:    map[string]string{"app": "kaniko", "submission-id": id8},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "dockerhub-secret"},
					},
					Containers: []corev1.Container{
						{
							Name:  "kaniko",
							Image: "gcr.io/kaniko-project/executor:latest",
							Args: []string{
								"--context=dir:///workspace/" + id8,
								"--destination=" + imageName,
								"--insecure",
								"--skip-tls-verify",
								"--registry-mirror=" + registryMirror,
							},
							Env: []corev1.EnvVar{
								{Name: "DOCKER_CONFIG", Value: "/kaniko/.docker"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace", ReadOnly: true},
								{Name: "docker-config", MountPath: "/kaniko/.docker", ReadOnly: true},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "submissions-pvc",
									ReadOnly:  true,
								},
							},
						},
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "dockerhub-secret",
									Items: []corev1.KeyToPath{
										{Key: ".dockerconfigjson", Path: "config.json"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	return err
}

func spawnSandboxPod(clientset *kubernetes.Clientset, id8, imageName, submissionID string) error {
	// Create Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-" + id8,
			Namespace: namespace,
			Labels: map[string]string{
				"app":           "sandbox",
				"submission-id": id8,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "sandbox",
					Image: imageName,
					Ports: []corev1.ContainerPort{{ContainerPort: 8080}},
					Env: []corev1.EnvVar{
						{Name: "SUBMISSION_ID", Value: submissionID},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot:             boolPtr(true),
						RunAsUser:                int64Ptr(1000),
						AllowPrivilegeEscalation: boolPtr(false),
					},
				},
			},
		},
	}

	_, err := clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create sandbox pod: %w", err)
	}

	// Create Service so bot-fleet can reach it
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-" + id8,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":           "sandbox",
				"submission-id": id8,
			},
			Ports: []corev1.ServicePort{
				{Port: 8080},
			},
		},
	}

	_, err = clientset.CoreV1().Services(namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	return err
}

func spawnBotFleetJob(clientset *kubernetes.Clientset, id8, submissionID string) error {
	ttl := int32(120)
	backoff := int32(0)
	targetURL := fmt.Sprintf("http://sandbox-%s:8080", id8)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bot-fleet-" + id8,
			Namespace: namespace,
			Labels:    map[string]string{"app": "bot-fleet", "submission-id": id8},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "bot-fleet"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "bot-fleet",
							Image:           "bot-fleet:" + os.Getenv("BOT_FLEET_IMAGE_TAG"),
							ImagePullPolicy: corev1.PullNever,
							Env: []corev1.EnvVar{
								{Name: "TARGET_URL", Value: targetURL},
								{Name: "SUBMISSION_ID", Value: submissionID},
								{Name: "NUM_BOTS", Value: "500"},
								{Name: "ORDERS_PER_BOT", Value: "100"},
								{Name: "REDIS_ADDR", Value: "redis:6379"},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	return err
}

func waitForJob(clientset *kubernetes.Clientset, jobName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if job.Status.Succeeded > 0 {
			return true, nil
		}
		if job.Status.Failed > 0 {
			return false, fmt.Errorf("job %s failed", jobName)
		}
		return false, nil
	})
}

func waitForPod(clientset *kubernetes.Clientset, podName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	})
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
