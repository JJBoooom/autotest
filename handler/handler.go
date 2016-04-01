package handler

import (
	"fmt"
	"net/http"

	docker "github.com/fsouza/go-dockerclient"
)

var (
	globalClient *DockerClient
)

type DockerClient struct {
	*docker.Client
	State int
}

//配合negroni,并且封装handler error
type JsonReturnHandler func(http.ResponseWriter, *http.Request) error

func (fn JsonReturnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func ListImages(http.ResponseWriter, *http.Request) error {
	endpoint := "unix://var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return err
	}
	imgs, err := client.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		return err
	}

	for _, img := range imgs {
		fmt.Println("ID: ", img.ID)
		fmt.Println("RepoTags: ", img.RepoTags)
		fmt.Println("Created: ", img.Created)
		fmt.Println("Size: ", img.Size)
		fmt.Println("VirtualSize: ", img.VirtualSize)
		fmt.Println("ParentId: ", img.ParentID)
	}
	return nil
}

func PullImage(http.ResponseWriter, *http.Request) error {
	image := "registry"
	ApiImages, err := globalClient.ListImages(docker.ListImagesOptions{Filter: image})
	if err != nil {
		return err
	}
	fmt.Println(ApiImages)
	return nil
}

func PushImage(http.ResponseWriter, *http.Request) error {
	return nil
}

func RemoveImage(http.ResponseWriter, *http.Request) error {
	return nil
}

//这里需要设置私有仓库地址,重启docker daemon, 在agent启动时,就要配置好
func init() {

	endpoint := "unix://var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}
	globalClient = &DockerClient{client, 0}

	//这个库,没有更新到1.20API,其中RegistryConfig字段没有
	/*
		dinfo, err := globlClient.Info()
		if err != nil {
			return err
		}

		fmt.Println(dinfo.Regis)
	*/

}
