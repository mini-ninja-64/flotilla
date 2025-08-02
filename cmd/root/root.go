package root

import (
	"github.com/mini-ninja-64/flotilla/cmd/sail"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var rootCommand = &cobra.Command{
		Use:   "flotilla",
		Short: "Flotilla lets you make multiple requests to all kubernetes pods in a service",
	}
	rootCommand.AddCommand(sail.Cmd())
	rootCommand.PersistentFlags().String("kubeconfig", "", "The kubeconfig file to use")
	rootCommand.PersistentFlags().String("context", "", "The context to use")
	rootCommand.PersistentFlags().StringP("namespace", "n", "", "The namespace to use")
	return rootCommand
}
