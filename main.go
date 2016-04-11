package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"test/handler"
	"test/routers"
	"time"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
)

var (
	ServerIP     string
	ServerPort   string
	ListenPort   string
	RegistryIp   string
	RegistryPort string
	log          = logrus.New()
)

func register(ip string, port string) (string, error) {
	client := new(BaseClient)
	client.Opts = new(ClientOpts)
	client.Opts.Url = "http://" + ip + ":" + port
	client.Opts.Timeout = time.Duration(10 * time.Second)

	resp, err := client.DoAction("/register/"+ListenPort, Get)

	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.New("register fail")
	} else {
		byteContent, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(byteContent), nil
	}
	return "", err
}

func ConfigRegistry(Registry string) error {

	if len(Registry) == 0 {
		log.Errorf("doesn't set registry")
		return errors.New("registry is empty ")
	}

	endpoint := "unix://var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		panic(err)
	}

	envs, err := client.Version()
	if err != nil {
		log.Errorf("get docker version fail:%v", err)
		return err
	}

	version := envs.Get("Version")
	if len(version) == 0 {
		log.Errorf("can't get version")
	}

	match, err := regexp.Match("(1\\.8.*)|(1.9.1)", []byte(version))
	if err != nil {
		return err
	}
	if match {
		docker_conf := "/etc/sysconfig/docker"
		//grep失败,则添加
		//这个正则替换得不到预期的效果

		//检测是否是新安装的docker
		cmd := fmt.Sprintf("grep -e \"^\\s*#\\s*INSECURE_REGISTRY.*--insecure-registry\\s*\" %s", docker_conf)
		fmt.Println(cmd)
		err := exec.Command("bash", "-c", cmd).Run()
		if err == nil {
			cmd = fmt.Sprintf(" sed -i \"s/^\\s*#\\s*INSECURE_REGISTRY=.*/INSECURE_REGISTRY= ' --insecure-registry %s '/\" %s", Registry, docker_conf)
			err := exec.Command("bash", "-c", cmd).Run()
			if err != nil {
				return errors.New("config registry fail: " + err.Error())
			}

		} else {
			if _, ok := err.(*exec.ExitError); ok {

				cmd := fmt.Sprintf("grep -e \"^\\s*INSECURE_REGISTRY.*--insecure-registry\\s*%s\" %s", Registry, docker_conf)
				fmt.Println(cmd)
				err := exec.Command("bash", "-c", cmd).Run()

				if err != nil {
					if _, ok := err.(*exec.ExitError); ok {
						//cmd = fmt.Sprintf(" sed -i \"s#^\\s*INSECURE_REGISTRY=\\s*'\\s*[-a-Z0-9\\.\\s]*#& --insecure-registry %s#\" %s", globalRegistry, docker_conf)
						cmd = fmt.Sprintf(" sed -i \"s#^\\s*INSECURE_REGISTRY=\\s*'\\s*[^']*#& --insecure-registry %s#\" %s", Registry, docker_conf)
						fmt.Println(cmd)
						err := exec.Command("bash", "-c", cmd).Run()

						if err != nil {
							return err
						}
					}
				}
			} else {
				return errors.New("config Registry fail: " + err.Error())
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
			command := fmt.Sprintf("grep -e \"^ExecStart=.*--insecure-registry %s\" %s ", Registry, docker_conf)
			log.Debug(command)
			err := exec.Command("bash", "-c", command).Run()

			if _, ok := err.(*exec.ExitError); ok {
				command = fmt.Sprintf("sed -i \"s#^ExecStart=.*#^& --insecure-registry %s\" %s", Registry, docker_conf)
				log.Debug(command)
				err := exec.Command("bash", "-c", command).Run()

				if err != nil {
					return err
				}
			}
		} else {
			panic("Unsupported Version")
		}
	}

	log.Info("ready to restart docker ..")
	command := fmt.Sprintln("systemctl restart docker")
	err = exec.Command("bash", "-c", command).Run()
	if err == nil {
		log.Info("restart done..")
	}

	return err
}

func main() {
	go func() {
		/*
			err := ConfigRegistry(RegistryIp + ":" + RegistryPort)
			if err != nil {
				panic("fail to config registry:" + err.Error())
			}
		*/

		//注册时,从上层服务器获取到registry(IP:Port)
		registryHost, err := register(ServerIP, ServerPort)
		if err != nil {
			panic(err)
		}
		if registryHost != (RegistryIp + ":" + RegistryPort) {
			panic("registry doesn't match server's registry")
		}
	}()
	log.Info("router..")
	router := routers.NewRouter()
	log.Info("listening on " + ListenPort)
	err := http.ListenAndServe(":"+ListenPort, router)
	if err != nil {

		panic(err)
	}
}

func init() {
	flag.StringVar(&ServerIP, "sip", "", "server ip")
	flag.StringVar(&ServerPort, "sport", "", "server port")
	flag.StringVar(&ListenPort, "lport", "", "listen port")
	flag.StringVar(&RegistryIp, "rip", "", "registry ip")
	flag.StringVar(&RegistryPort, "rport", "", "registry port")

	flag.Parse()

	if len(ServerIP) == 0 || len(ServerPort) == 0 || len(ListenPort) == 0 || len(RegistryIp) == 0 || len(RegistryPort) == 0 {
		panic("invalid argument")
	}
	handler.SetRegistry(RegistryIp + ":" + RegistryPort)
}
