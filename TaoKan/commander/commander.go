package commander

import (
	"errors"
	"fmt"
	"github.com/gliderlabs/ssh"
	"github.com/melbahja/goph"
	log "github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
	"io"
	"strings"
	"sync"
)

var KubeConfig string
var Namespace string

var lock = &sync.Mutex{}

type Mode string

const (
	ServerMode Mode = "server"
	ClientMode Mode = "client"
)

const welcomeMsg = "[TaoKan Server]\n"

var clientInstance *Commander
var serverInstance *Commander

type Action struct {
	Names      []string
	ServerFunc func(w io.Writer, args []string) error
}

var actions = []Action{
	{
		Names:      []string{"status"},
		ServerFunc: status,
	},
	{
		Names:      []string{"mount"},
		ServerFunc: mountPvc,
	},
	{
		Names:      []string{"unmount", "umount"},
		ServerFunc: umountPvc,
	},
}

type Commander struct {
	Remote  string
	Port    uint
	Mode    Mode
	Actions []Action

	client *goph.Client
}

type Config struct {
	Namespace  string
	KubeConfig string
	Remote     string
	Port       uint
}

func serverCommandDispatcher(c *Commander, w io.Writer, commands []string) error {
	if len(commands) == 0 {
		return errors.New("[Error] No command provided.")
	}
	cmd := commands[0]
	for _, action := range c.Actions {
		for _, name := range action.Names {
			if name == cmd {
				err := action.ServerFunc(w, commands[1:])
				if err != nil {
					return err
				}
				return nil
			}
		}
	}
	return errors.New("Unsupported command '" + cmd + "'")
}

func clientCommandDispatcher(c *Commander, command string, args []string) (string, error) {
	log.Infof("[Run] Command: `%s`", command)
	if command == "" {
		return "", errors.New("[Error] No command provided.")
	}
	cmd := fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	outBytes, err := c.client.Run(cmd)
	output := string(outBytes)
	return output, err
}

func StartServer(config Config) error {
	commander := &Commander{
		Port:    config.Port,
		Mode:    ServerMode,
		Actions: actions,
	}
	KubeConfig = config.KubeConfig
	Namespace = config.Namespace
	ssh.Handle(func(s ssh.Session) {
		io.WriteString(s, welcomeMsg)
		log.Infof("[Receive] Command: `%s`", strings.Join(s.Command(), " "))
		err := serverCommandDispatcher(commander, s, s.Command())
		if err != nil {
			io.WriteString(s, "[Error] "+err.Error())
			log.Error(err)
			s.Close()
		}
		log.Infof("[Closed] Command: `%s`", strings.Join(s.Command(), " "))
	})
	addr := fmt.Sprintf(":%d", config.Port)
	go log.Fatal(ssh.ListenAndServe(addr, nil))
	return nil
}

func StartClient(config Config) (*Commander, error) {
	if clientInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		clientInstance = &Commander{
			Port:    config.Port,
			Remote:  config.Remote,
			Mode:    ClientMode,
			Actions: actions,
		}
		KubeConfig = config.KubeConfig
		Namespace = config.Namespace

		auth, _ := goph.UseAgent()
		sshConfig := &goph.Config{
			User:     "rsync",
			Addr:     config.Remote,
			Port:     config.Port,
			Auth:     auth,
			Timeout:  goph.DefaultTimeout,
			Callback: gossh.InsecureIgnoreHostKey(),
		}
		client, err := goph.NewConn(sshConfig)
		if err != nil {
			log.Fatal(err)
		}
		clientInstance.client = client
	}
	return clientInstance, nil
}

func (c *Commander) Close() {
	if c.Mode == ClientMode {
		lock.Lock()
		defer lock.Unlock()
		c.client.Close()
		clientInstance = nil
	}
}

func (c *Commander) Run(cmd string, args ...string) (string, error) {
	return clientCommandDispatcher(c, cmd, args)
}
