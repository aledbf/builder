package gitreceive

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"text/template"
	"time"

	"github.com/deis/pkg/log"

	"github.com/docker/distribution/context"
	"k8s.io/kubernetes/pkg/api"

	"github.com/deis/builder/pkg/git"
	"github.com/deis/builder/pkg/storage"
	docker "github.com/fsouza/go-dockerclient"
)

const (
	endpoint = "unix:///var/run/docker.sock"

	appTemplate = `FROM {{ .baseImage }}

ENV GIT_SHA {{ .gitSHA }}

`
)

var appTemplateTpl = template.Must(template.New("appTemplate").Parse(appTemplate))

type buildContext struct {
	AppName       string
	Sha           *git.SHA
	ArtifactPath  string
	Username      string
	Password      string
	ServerAddress string
}

func buildImage(getter storage.ObjectGetter, bc *buildContext) (string, error) {
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return "", err
	}

	var hookByteBuf bytes.Buffer
	err = appTemplateTpl.Execute(&hookByteBuf, map[string]string{
		"baseImage": os.Getenv("SLUGRUNNER_IMAGE_NAME"),
		"gitSHA":    bc.Sha.Full(),
	})
	if err != nil {
		return "", err
	}

	tempDir, err := ioutil.TempDir("", fmt.Sprintf("build-app-%v", time.Now().UnixNano()))
	if err != nil {
		return "", fmt.Errorf("unexpected error creating temporal directory %v", err)
	}

	response, err := getter.GetContent(context.Background(), bc.ArtifactPath)
	if err != nil {
		return "", fmt.Errorf("Error while downloading %v, %v", bc.ArtifactPath, err)
	}

	if err := ioutil.WriteFile(tempDir+"/slug.tgz", response, 0644); err != nil {
		return "", fmt.Errorf("Error while creating slug.tgz (%v)", err)
	}

	err = ioutil.WriteFile(tempDir+"/Dockerfile", hookByteBuf.Bytes(), 0644)
	if err != nil {
		return "", err
	}

	tagName := "git-" + bc.Sha.Short()
	dockerImage := getImageNameTag(bc.AppName, tagName)

	log.Info("building docker image %v", dockerImage)
	log.Debug("dockerfile: %s", hookByteBuf.Bytes())

	opts := docker.BuildImageOptions{
		Name:           dockerImage,
		ContextDir:     tempDir,
		RmTmpContainer: true,
		OutputStream:   os.Stdout,
	}

	err = client.BuildImage(opts)
	if err != nil {
		return "", err
	}

	dAuth := docker.AuthConfiguration{}

	if bc.ServerAddress != "" {
		dAuth.Username = bc.Username
		dAuth.Password = bc.Password
		dAuth.ServerAddress = bc.ServerAddress
	}

	log.Info("publishing docker image")
	var buf bytes.Buffer
	err = client.PushImage(docker.PushImageOptions{
		Name:         getImageName(bc.AppName),
		Tag:          tagName,
		OutputStream: &buf,
	}, dAuth)

	log.Debug("%s", buf.Bytes())

	if err != nil {
		return "", fmt.Errorf("unexpected error publishing docker image: %v", err)
	}

	return dockerImage, nil
}

func getImageName(appName string) string {
	host := os.Getenv("DEIS_REGISTRY_SERVICE_HOST")
	port := os.Getenv("DEIS_REGISTRY_SERVICE_PORT")

	return fmt.Sprintf("%v:%v/%v", host, port, appName)
}

func getImageNameTag(appName, tagName string) string {
	return fmt.Sprintf("%v:%v", getImageName(appName), tagName)
}

func getAuthFromCfg(data string) (*docker.AuthConfigurations, error) {
	return docker.NewAuthConfigurations(bytes.NewReader([]byte(data)))
}

func authRegistry(host string, secrets []api.Secret) (docker.AuthConfiguration, error) {
	for _, secret := range secrets {
		data := secret.Data[api.DockerConfigKey]
		auth, err := getAuthFromCfg(string(data))
		if err != nil {
			return docker.AuthConfiguration{}, err
		}

		for key, value := range auth.Configs {
			pHost, err := url.Parse(key)
			if err != nil {
				continue
			}
			if pHost.Host == host {
				return value, nil
			}
		}
	}

	return docker.AuthConfiguration{}, fmt.Errorf("No docker secrets found for host %v", host)
}
