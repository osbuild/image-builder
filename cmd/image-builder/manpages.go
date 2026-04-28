package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func addDocCmd(rootCmd *cobra.Command) {
	docCmd := &cobra.Command{
		Use:    "doc <output-dir>",
		Short:  "Generate man pages for this command",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			header := &doc.GenManHeader{
				Section: "1",
			}
			return doc.GenManTree(rootCmd, header, args[0])
		},
	}
	rootCmd.AddCommand(docCmd)
}
