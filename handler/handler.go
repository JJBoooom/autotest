package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strings"
	"test/errjson"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/gorilla/mux"
)

var (
	globalClient *DockerClient
	//在服务器启动后,询问上级服务器获取registry的地址
	globalRegistry string
	log            = logrus.New()
	ui             = new(UserInfo)
	logFile        = "./test.log"
)

type DockerClient struct {
	*docker.Client
	State int
}

func SetRegistry(registry string) {
	globalRegistry = registry
}

//配合negroni,并且封装handler error
type JsonReturnHandler func(http.ResponseWriter, *http.Request) error

func (fn JsonReturnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		log.Error(err)
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
	ApiImages, err := globalClient.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		return docker.APIImages{}, err
	}

	for _, v := range ApiImages {
		for i := 0; i < len(v.RepoTags); i++ {
			//			log.Debugf("getImage:[%s]", v.RepoTags[i])
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
		log.Errorf("ListImages:%v\n", err)
		return err
	}

	fmt.Fprintf(w, string(byteContent))
	log.Debugf("ListImages:success\n")
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
		log.Errorf("pushFromPublic check image[%s:%s] exists fail:%v\n", image, tag, err)
		return errjson.NewInternalServerError(err.Error())
	}

	if exists {
		return nil
	}

	slice := strings.SplitN(image, "/", 2)
	registry := slice[0]
	repo := slice[1]

	opts := docker.PullImageOptions{
		Repository: repo,
		Tag:        tag,
		Registry:   registry,
	}
	auths := docker.AuthConfiguration{}
	err = globalClient.PullImage(opts, auths)
	if err != nil {
		log.Errorf("pushFromPublic: pull image[%s:%s] fail:%v\n", image, tag, err)
		return err
	}
	log.Debugf("pushFromPublic success")
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

	json.Unmarshal(byteContent, &tagOpt)
	old := tagOpt.Old
	new := tagOpt.New

	slice1 := strings.Split(old, ":")
	tag1 := slice1[len(slice1)-1]

	image1 := slice1[0]
	for i := 1; i < len(slice1)-1; i++ {
		image1 = image1 + ":" + slice1[i]
	}

	slice2 := strings.Split(new, ":")
	tag2 := slice2[len(slice2)-1]
	image2 := slice2[0]
	for i := 1; i < len(slice2)-1; i++ {
		image2 = image2 + ":" + slice2[i]
	}

	log.Debugf("TagImage : image:%v, tag:%v\n", image2, tag2)

	exists, err := IsImageExist(image1, tag1)
	if err != nil {
		log.Errorf("[%s:%s] get Image exist check fail:%v \n", image1, tag1, err)
		return errjson.NewInternalServerError(err.Error())
	}

	if !exists {
		Msg := fmt.Sprintf("image[%s] doesn't exists\n", old)
		log.Errorf(Msg)
		return errors.New(Msg)
	}

	opts := docker.TagImageOptions{
		Tag:   tag2,
		Repo:  image2,
		Force: true, //不设置的话，如果镜像存在，Tag将失败
	}
	err = globalClient.TagImage(old, opts)
	if err != nil {
		t := reflect.TypeOf(err)
		log.Errorf("Tag [%v==>%v:%v ] ErrType[%v:%v] fail:%v\n", old, image2, tag2, t.Name(), t.String(), err)
	}
	if err != nil {
		log.Errorf("TagImage [%v ==> %v:%v] fail for %v", old, image2, tag2, err.Error())
	} else {
		log.Infof("TagImage [%v ==> %v:%v] success", old, image2, tag2)
	}
	return err
}

func CheckExists(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	image := vars["image"]
	tag := vars["tag"]

	if len(image) == 0 || len(tag) == 0 {
		return errors.New("invalid argument")
	}
	exists, err := IsImageExist(image, tag)
	if err != nil {
		log.Errorf("PullImage check image [%s:%s] exists fail:%v\n", image, tag, err)
		return errjson.NewInternalServerError(err.Error())
	}

	if exists {
		fmt.Fprintf(w, "exists")
	} else {
		fmt.Fprintf(w, "not exists")
	}
	return nil
}

type UserInfo struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Server   string `json:"server"`
}

func Login(w http.ResponseWriter, r *http.Request) error {

	byteContent, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer func() {
		if r.Body != nil {
			r.Body.Close()
		}
	}()

	err = json.Unmarshal(byteContent, &ui)
	if err != nil {
		return err
	}

	log.Debugf("user: %s\n", ui.User)
	log.Debugf("Password: %s\n", ui.Password)
	log.Debugf("server: %s\n", ui.Server)

	return nil
}

func PullImage(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	image := vars["image"]
	tag := vars["tag"]

	if len(image) == 0 || len(tag) == 0 {
		return errors.New("invalid argument")
	}

	opts := docker.PullImageOptions{
		Repository: image,
		Tag:        tag,
		Registry:   globalRegistry,
	}
	auths := docker.AuthConfiguration{
		Username:      ui.User,
		Password:      ui.Password,
		ServerAddress: ui.Server,
	}
	err := globalClient.PullImage(opts, auths)
	if err != nil {
		t := reflect.TypeOf(err)
		log.Errorf("PullImage:[%s:%s] ErrType:[%s:%s] fail:%v\n", image, tag, t.Name(), t.String(), err)
	} else {
		log.Debugf("PullImage:[%s:%s] success", image, tag)
	}

	return err
}

func PushImage(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	image := vars["image"]
	tag := vars["tag"]

	if len(image) == 0 || len(tag) == 0 {
		return errors.New("invalid argument")
	}
	log.Debugf("pushImage:[%s:%s]\n", image, tag)

	exists, err := IsImageExist(image, tag)
	if err != nil {
		log.Errorf("pushImage:check[%s:%s] exists fail:%v", image, tag, err)
		return err
	}

	if !exists {
		Msg := fmt.Sprintf("%v:%v doesn't exist", image, tag)
		log.Errorf(Msg)
		return errjson.NewErrForbidden(Msg)
	}

	opts := docker.PushImageOptions{
		Name: image,
		Tag:  tag,
	}
	auth := docker.AuthConfiguration{
		Username:      ui.User,
		Password:      ui.Password,
		ServerAddress: ui.Server,
	}
	err = globalClient.PushImage(opts, auth)
	if err != nil {
		t := reflect.TypeOf(err)
		log.Errorf("pushImage:[%s:%s] ErrType:[%v:%v] fail:%v\n", image, tag, t.Name(), t.String(), err)
		return errjson.NewInternalServerError(err.Error())
	}

	log.Debugf("pushImage [%s:%s] success", image, tag)
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

func Shutdown(http.ResponseWriter, *http.Request) {
	log.Info("recieve server's shutdown command")
	os.Exit(0)
}

func init() {

	log.Level = logrus.DebugLevel

	fp, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) // 让程序关闭后,自动释放
	if err != nil {
		log.Errorf("Create logfile %s fail\n", logFile)
		os.Exit(1)
	}
	log.Out = fp

	endpoint := "unix://var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}
	globalClient = &DockerClient{client, 0}

}
