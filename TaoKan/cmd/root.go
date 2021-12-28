package cmd

import (
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var KubeConfig string
var Namespace string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "TaoKan",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.TaoKan.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	kubeconfig := ""
	home := homedir.HomeDir()
	if home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	rootCmd.PersistentFlags().StringVar(&KubeConfig, "kubeconfig", kubeconfig, "absolute path to the kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&Namespace, "namespace", "n", "hub", "default namespace of k8s")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable Debug verbose")
}
