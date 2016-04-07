package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"test/errjson"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gorilla/mux"
)

var (
	globalClient   *DockerClient
	globalRegistry = "192.168.15.83:5000" //在服务器启动后,询问上级服务器获取registry的地址

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
		if _, ok := err.(notfound); ok {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

type notfound struct {
	msg string
}

func (e notfound) Error() string {
	return e.msg
}

func getImage(image string, tag string) (docker.APIImages, error) {
	ApiImages, err := globalClient.ListImages(docker.ListImagesOptions{Filter: image})
	if err != nil {
		return docker.APIImages{}, err
	}

	for _, v := range ApiImages {
		for i := 0; i < len(v.RepoTags); i++ {
			if v.RepoTags[i] == image+":"+tag {
				return v, nil
			}
		}
	}

	return docker.APIImages{}, notfound{msg: "not found"}
}

type ImageList struct {
	Image string `json:"image"`
}

func ListImages(w http.ResponseWriter, r *http.Request) error {

	imgs, err := globalClient.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		return err
	}

	var imagelist []ImageList
	for _, img := range imgs {
		for _, j := range img.RepoTags {
			newImage := new(ImageList)
			newImage.Image = j
			imagelist = append(imagelist, *newImage)
		}
	}
	byteContent, err := json.Marshal(imagelist)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, string(byteContent))

	return nil
}

func PublicPullImage(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	image := vars["image"]
	tag := vars["tag"]

	if len(image) == 0 || len(tag) == 0 {
		return errors.New("invalid argument")
	}

	exists, err := IsImageExist(image, tag)
	if err != nil {
		return errjson.NewInternalServerError(err.Error())
	}

	if exists {
		return nil
	}
	opts := docker.PullImageOptions{
		Tag: tag,
	}
	auths := docker.AuthConfiguration{}
	globalClient.PullImage(opts, auths)

	return nil
}

type TagOpt struct {
	New string `json:"new"`
	Old string `json:"old"`
}

func TagImage(w http.ResponseWriter, r *http.Request) error {
	var tagOpt TagOpt

	byteContent, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {

		return err
	}
	fmt.Println("tagImage: bytecontent:" + string(byteContent))

	json.Unmarshal(byteContent, &tagOpt)
	old := tagOpt.Old
	new := tagOpt.New

	slice1 := strings.Split(old, ":")
	tag1 := slice1[len(slice1)-1]

	image1 := slice1[0]
	for i := 1; i < len(slice1)-1; i++ {
		image1 = image1 + ":" + slice1[i]
	}

	// 192.168.15.119:5000/registry:2.1
	slice2 := strings.Split(new, ":")
	tag2 := slice2[len(slice2)-1]
	image2 := slice2[0]
	for i := 1; i < len(slice2)-1; i++ {
		image2 = image2 + ":" + slice2[i]
	}

	fmt.Printf("TagImage : image:%v, tag:%v\n", image2, tag2)

	exists, err := IsImageExist(image1, tag1)
	if err != nil {
		fmt.Printf("%s:%s get Image check ", image1, tag1)
		return errjson.NewInternalServerError(err.Error())
	}

	if !exists {
		Msg := fmt.Sprintf("image[%s] doesn't exists\n", old)
		fmt.Println(Msg)
		return errors.New(Msg)
	}

	//这里需要确认这个命令需要传递什么样的参数
	opts := docker.TagImageOptions{
		Tag:   tag2,
		Repo:  image2,
		Force: true, //不设置的话，如果镜像存在，Tag将失败
	}
	err = globalClient.TagImage(old, opts)
	if err != nil {
		fmt.Printf("Tag [%v==>%v:%v fail]:%v", old, image2, tag2, err)
	}
	return err
}

func PullImage(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	image := vars["image"]
	tag := vars["tag"]

	if len(image) == 0 || len(tag) == 0 {
		return errors.New("invalid argument")
	}

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
	err = globalClient.PullImage(opts, auths)
	return err
}

func PushImage(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	image := vars["image"]
	tag := vars["tag"]

	if len(image) == 0 || len(tag) == 0 {
		return errors.New("invalid argument")
	}
	fmt.Println(image + ":" + tag)

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

func ConfigRegistry(Registry string) error {

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
		cmd := fmt.Sprintf("grep -e \"^\\s*INSECURE_REGISTRY.*--insecure-registry\\s*%s\" %s", globalRegistry, docker_conf)
		fmt.Println(cmd)
		err := exec.Command("bash", "-c", cmd).Run()

		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				//cmd = fmt.Sprintf(" sed -i \"s#^\\s*INSECURE_REGISTRY=\\s*'\\s*[-a-Z0-9\\.\\s]*#& --insecure-registry %s#\" %s", globalRegistry, docker_conf)
				cmd = fmt.Sprintf(" sed -i \"s#^\\s*INSECURE_REGISTRY=\\s*'\\s*[^']*#& --insecure-registry %s#\" %s", globalRegistry, docker_conf)
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
	if err == nil {
		fmt.Println("restart done..")
	}

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
	err = ConfigRegistry(globalRegistry)
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
