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
)

var KubeConfig string
var Namespace string

type Mode string

const (
	ServerMode Mode = "server"
	ClientMode Mode = "client"
)

const welcomeMsg = "[TaoKan Server]\n"

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
		return errors.New("[Error] No command provided.\n")
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
	return errors.New("Unsupported command '" + cmd + "'\n")
}

func clientCommandDispatcher(c *Commander, command string, args []string) (string, error) {
	log.Infof("[Run] command '%s'\n", command)
	if command == "" {
		return "", errors.New("[Error] No command provided.\n")
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
		log.Infoln("[Receive] Command", s.Command())
		err := serverCommandDispatcher(commander, s, s.Command())
		if err != nil {
			io.WriteString(s, "[Error] "+err.Error()+"\n")
			log.Error(err)
		}
		log.Infoln("[Closed]", s.Command())
	})
	addr := fmt.Sprintf(":%d", config.Port)
	go log.Fatal(ssh.ListenAndServe(addr, nil))
	return nil
}

func StartClient(config Config) (*Commander, error) {
	commander := &Commander{
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
	commander.client = client

	return commander, nil
}

func (c *Commander) Close() {
	if c.Mode == ClientMode {
		c.client.Close()
	}
}

func (c *Commander) Run(cmd string, args ...string) (string, error) {
	return clientCommandDispatcher(c, cmd, args)
}
