package gitreceive

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pborman/uuid"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	slugBuilderName   = "deis-slugbuilder"
	dockerBuilderName = "deis-dockerbuilder"

	tarPath          = "TAR_PATH"
	putPath          = "PUT_PATH"
	debugKey         = "DEIS_DEBUG"
	objectStore      = "objectstorage-keyfile"
	dockerSocketName = "docker-socket"
	dockerSocketPath = "/var/run/docker.sock"
	builderStorage   = "BUILDER_STORAGE"
	objectStorePath  = "/var/run/secrets/deis/objectstore/creds"
)

func dockerBuilderPodName(appName, shortSha string) string {
	uid := uuid.New()[:8]
	return fmt.Sprintf("dockerbuild-%s-%s-%s", appName, shortSha, uid)
}

func slugBuilderPodName(appName, shortSha string) string {
	uid := uuid.New()[:8]
	return fmt.Sprintf("slugbuild-%s-%s-%s", appName, shortSha, uid)
}

func dockerBuilderPod(
	debug bool,
	name,
	namespace string,
	env map[string]interface{},
	tarKey,
	imageName,
	storageType,
	image string,
	pullPolicy api.PullPolicy,
) *api.Pod {

	pod := buildPod(debug, name, namespace, pullPolicy, env)

	pod.Spec.Containers[0].Name = dockerBuilderName
	pod.Spec.Containers[0].Image = image

	addEnvToPod(pod, tarPath, tarKey)
	addEnvToPod(pod, "IMG_NAME", imageName)
	addEnvToPod(pod, builderStorage, storageType)

	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, api.VolumeMount{
		Name:      dockerSocketName,
		MountPath: dockerSocketPath,
	})

	pod.Spec.Volumes = append(pod.Spec.Volumes, api.Volume{
		Name: dockerSocketName,
		VolumeSource: api.VolumeSource{
			HostPath: &api.HostPathVolumeSource{
				Path: dockerSocketPath,
			},
		},
	})

	pod.ObjectMeta.Labels["buildType"] = "dockerBuilder"

	return &pod
}

func slugbuilderPod(
	debug bool,
	name,
	namespace string,
	env map[string]interface{},
	tarKey,
	putKey,
	buildpackURL,
	storageType,
	image string,
	pullPolicy api.PullPolicy,
) *api.Pod {

	pod := buildPod(debug, name, namespace, pullPolicy, env)

	pod.Spec.Containers[0].Name = slugBuilderName
	pod.Spec.Containers[0].Image = image

	addEnvToPod(pod, tarPath, tarKey)
	addEnvToPod(pod, putPath, putKey)
	addEnvToPod(pod, builderStorage, storageType)

	if buildpackURL != "" {
		addEnvToPod(pod, "BUILDPACK_URL", buildpackURL)
	}

	pod.ObjectMeta.Labels["buildType"] = "slugBuilder"

	return &pod
}

func buildPod(
	debug bool,
	name,
	namespace string,
	pullPolicy api.PullPolicy,
	env map[string]interface{}) api.Pod {

	pod := api.Pod{
		Spec: api.PodSpec{
			RestartPolicy: api.RestartPolicyNever,
			Containers: []api.Container{
				api.Container{
					ImagePullPolicy: pullPolicy,
				},
			},
			Volumes: []api.Volume{},
		},
		ObjectMeta: api.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage": name,
			},
		},
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, api.Volume{
		Name: objectStore,
		VolumeSource: api.VolumeSource{
			Secret: &api.SecretVolumeSource{
				SecretName: objectStore,
			},
		},
	})

	pod.Spec.Containers[0].VolumeMounts = []api.VolumeMount{
		api.VolumeMount{
			Name:      objectStore,
			MountPath: objectStorePath,
			ReadOnly:  true,
		},
	}

	if len(pod.Spec.Containers) > 0 {
		for k, v := range env {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, api.EnvVar{
				Name:  k,
				Value: fmt.Sprintf("%v", v),
			})
		}
	}

	if debug {
		addEnvToPod(pod, debugKey, "1")
	}

	return pod
}

func addEnvToPod(pod api.Pod, key, value string) {
	if len(pod.Spec.Containers) > 0 {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, api.EnvVar{
			Name:  key,
			Value: value,
		})
	}
}

func progress(msg string, interval time.Duration) chan bool {
	tick := time.Tick(interval)
	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-quit:
				close(quit)
				return
			case <-tick:
				fmt.Println(msg)
			}
		}
	}()
	return quit
}

func getImagePullSecrets(c *client.Client, namespace string) ([]api.Secret, error) {
	pullSecrets := os.Getenv("PULL_SECRETS")
	if pullSecrets == "" {
		return []api.Secret{}, nil
	}

	secrets := []api.Secret{}

	secretsNames := strings.Split(pullSecrets, ",")
	for _, secretName := range secretsNames {
		secret, err := c.Secrets(namespace).Get(secretName)
		if err != nil {
			continue
		}

		if secret.Type == api.SecretTypeDockercfg {
			secrets = append(secrets, *secret)
		}
	}

	return secrets, nil
}
