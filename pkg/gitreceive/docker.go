package gitreceive

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"text/template"

	"github.com/deis/pkg/log"

	"github.com/deis/builder/pkg/gitreceive/git"
	"github.com/fsouza/go-dockerclient"
)

const (
	endpoint   = "unix:///var/run/docker.sock"
	slugRunner = "dev-registry.soficom.cl/deis/slugrunner:v1.0.1"

	appTemplate = `FROM {{ .baseImage }}

ENV GIT_SHA {{ .gitSHA }}

`
)

var appTemplateTpl = template.Must(template.New("appTemplate").Parse(appTemplate))

type buildContext struct {
	AppName string
	Sha     *git.SHA
	Tgz     string
}

func buildImage(context *buildContext) (string, error) {
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return "", err
	}

	var hookByteBuf bytes.Buffer
	err = appTemplateTpl.Execute(&hookByteBuf, map[string]string{
		"baseImage": slugRunner,
		"tgzURL":    context.Tgz,
		"gitSHA":    context.Sha.Full(),
	})
	if err != nil {
		return "", err
	}

	tempDir, err := ioutil.TempDir("", "build-app")
	if err != nil {
		return "", fmt.Errorf("unexpected error creating temporal directory %v", err)
	}

	output, err := os.Create(tempDir + "/slug.tgz")
	if err != nil {
		return "", fmt.Errorf("Error while creating file %v", err)
	}
	defer output.Close()

	response, err := http.Get(context.Tgz)
	if err != nil {
		return "", fmt.Errorf("Error while downloading", context.Tgz, "-", err)
	}
	defer response.Body.Close()

	_, err = io.Copy(output, response.Body)
	if err != nil {
		return "", fmt.Errorf("Error while downloading slug.tgz (%v)", err)
	}

	err = ioutil.WriteFile(tempDir+"/Dockerfile", hookByteBuf.Bytes(), 0644)
	if err != nil {
		return "", err
	}

	tagName := "git-" + context.Sha.Short()
	dockerImage := getImageNameTag(context.AppName, tagName)

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

	log.Info("publishing docker image")
	err = client.PushImage(docker.PushImageOptions{
		Name:         getImageName(context.AppName),
		Tag:          tagName,
		OutputStream: os.Stdout,
	}, docker.AuthConfiguration{})
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
