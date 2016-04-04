package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"test/errjson"

	docker "github.com/fsouza/go-dockerclient"
)

var (
	globalClient   *DockerClient
	globalRegistry = "192.168.15.86:5000" //在服务器启动后,询问上级服务器获取registry的地址

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

func IsImageExist(image string, tag string) (bool, error) {
	_, err := getImage(image, tag)
	if err != nil {
		return false, err
	}

	return true, nil
}

func getImage(image string, tag string) (docker.APIImages, error) {
	ApiImages, err := globalClient.ListImages(docker.ListImagesOptions{Filter: image})
	if err != nil {
		return docker.APIImages{}, err
	}

	for _, v := range ApiImages {
		for i := 0; i < len(v.RepoTags); i++ {
			if v.RepoTags[i] == tag {
				return v, nil
			}
		}
	}

	return docker.APIImages{}, errors.New("Image not found")
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
	tag := "2"

	exists, err := IsImageExist(image, tag)
	if err != nil {
		return errjson.NewInternalServerError(err.Error())
	}

	if exists {
		return nil
	}
	opts := docker.PullImageOptions{
		Tag:      tag,
		Registry: globalRegistry,
	}
	auths := docker.AuthConfiguration{}
	globalClient.PullImage(opts, auths)

	/*
		for _, v := range ApiImages {
			fmt.Printf("ID:%v\n", v.ID)
			fmt.Printf("ParentID:%v\n", v.ParentID)
			fmt.Printf("Size:%v\n", v.Size)
			fmt.Printf("VirtualSize:%v\n", v.VirtualSize)
			fmt.Printf("Tags:%v\n", v.RepoTags)
			fmt.Printf("Created:%v\n", v.Created)
			fmt.Printf("Lables:%v\n", v.Labels)
			fmt.Println("-----------------------")
		}
	*/
	return nil
}

func PushImage(http.ResponseWriter, *http.Request) error {
	image := "registry"
	tag := "2"
	exists, err := IsImageExist(image, tag)
	if err != nil {
		return err
	}

	if !exists {
		Msg := fmt.Sprintf("%v:%v doesn't exist", image, tag)
		fmt.Printf(Msg)
		//Forbidden 错误
		return errjson.NewErrForbidden(Msg)
	}

	opts := docker.PushImageOptions{
		Name:     image,
		Tag:      tag,
		Registry: globalRegistry,
	}
	auth := docker.AuthConfiguration{}
	err = globalClient.PushImage(opts, auth)
	if err != nil {
		fmt.Println(err)
		return errjson.NewInternalServerError(err.Error())
	}

	return nil
}

func removeImage(image string, tag string) error {
	imageInfo, err := getImage(image, tag)
	if err != nil {
		return err
	}
	//需要测试是否通过imageTag的ID能够删除指定的tag
	err = globalClient.RemoveImage(imageInfo.ID)
	return err
}

func RemoveImage(http.ResponseWriter, *http.Request) error {
	image := "registry"
	tag := "2"

	err := removeImage(image, tag)
	return err
}

//这里需要设置私有仓库地址,重启docker daemon, 在agent启动时,就要配置好

func configRegistry(Registry string) error {

	if len(Registry) == 0 {
		return errors.New("registry is empty ")
	}

	envs, err := globalClient.Version()
	if err != nil {
		return err
	}

	version := envs.Get("Version")
	if len(version) == 0 {
		panic("can't get version")
	}

	match, err := regexp.Match("1\\.8.*", []byte(version))
	if err != nil {
		return err
	}
	if match {
		docker_conf := "/etc/sysconfig/docker"
		//grep失败,则添加
		//这个正则替换得不到预期的效果
		cmd := fmt.Sprintf("grep -e \"^\\s*INSECURE_REGISTRY.*--insecure-registry\\s*%s %s", globalRegistry, docker_conf)
		fmt.Println(cmd)
		err := exec.Command("bash", "-c", cmd).Run()

		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				cmd = fmt.Sprintf(" sed -i \"s#^\\s*INSECURE_REGISTRY=\\s*'\\s*[-a-Z0-9.\\s]*#& --insecure-registry %s'#\" %s", globalRegistry, docker_conf)
				fmt.Println(cmd)
				err := exec.Command("bash", "-c", cmd).Run()

				if err != nil {
					return err
				}
			}
		}
	} else {
		docker_conf := "/usr/lib/systemd/system/docker.service"
		//1.10.*或1.9.*版本的配置文件
		match, err := regexp.Match("1\\.([10 | 9]).*", []byte(version))
		if err != nil {
			return err
		}

		if match {
			command := fmt.Sprintf("grep -e \"^ExecStart=.*--insecure-registry %s\" %s ", globalRegistry, docker_conf)
			fmt.Println(command)
			err := exec.Command("bash", "-c", command).Run()

			if _, ok := err.(*exec.ExitError); ok {
				command = fmt.Sprintf("sed -i \"s#^ExecStart=.*#^& --insecure-registry %s\" %s", globalRegistry, docker_conf)
				fmt.Println(command)
				err := exec.Command("bash", "-c", command).Run()

				if err != nil {
					return err
				}
			}
		} else {
			panic("Unsupported Version")
		}
	}

	fmt.Println("ready to restart docker ..")
	command := fmt.Sprintln("systemctl restart docker")
	err = exec.Command("bash", "-c", command).Run()

	return err
}

func init() {
	/*
		log.SetLevel(log.DebugLevel)

		var logicServer string
		flag.StringVar(&logicServer, "lserver", "", "<ip>:<port> of logic server")
		flag.StringVar(&registryServer, "rserver", "", "<ip>:<port> of registry server")

		if len(logicServer) == 0 {
			log.Fatal("must set logic server ip:port")
		}
		if len(registryServer) == 0 {
			log.Fatal("must set registry server ip:port")
		}
	*/

	endpoint := "unix://var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}
	globalClient = &DockerClient{client, 0}

	fmt.Println("config registry ...")
	err = configRegistry(globalRegistry)
	if err != nil {
		panic(err)
	}
	//这个库,没有更新到1.20API,其中RegistryConfig字段没有
	/*
		dinfo, err := globlClient.Info()
		if err != nil {
			return err
		}

		fmt.Println(dinfo.Regis)
	*/

}
