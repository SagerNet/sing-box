package main

import (
	"io"
	"os"

	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/log"
	"github.com/spf13/cobra"
)

var (
	profileEncodeOutput string
	profileDecodeOutput string
)

var commandProfile = &cobra.Command{
	Use:   "profile",
	Short: "Profile encoder and decoder",
}

func init() {
	commandTools.AddCommand(commandProfile)
	commandProfile.AddCommand(commandProfileEncode)
	commandProfile.AddCommand(commandProfileDecode)

	commandProfileEncode.Flags().StringVar(&profileEncodeOutput, "output", "sing-box.profile", "Output file name")
	commandProfileDecode.Flags().StringVar(&profileDecodeOutput, "output", "sing-box.json", "Output file name")
}

var commandProfileEncode = &cobra.Command{
	Use:   "encode",
	Short: "Encode profile from stdin",
	Run: func(cmd *cobra.Command, args []string) {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}

		p := &libbox.ProfileContent{
			Name:   "imported-from-file",
			Type:   libbox.ProfileTypeLocal,
			Config: string(content),
		}

		if err := os.WriteFile(profileEncodeOutput, p.Encode(), os.ModePerm); err != nil {
			log.Fatal(err)
		}
	},
}

var commandProfileDecode = &cobra.Command{
	Use:   "decode",
	Short: "Decode profile from stdin",
	Run: func(cmd *cobra.Command, args []string) {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}

		profile, err := libbox.DecodeProfileContent(content)
		if err != nil {
			log.Fatal(err)
		}

		if err := os.WriteFile(profileDecodeOutput, []byte(profile.Config), os.ModePerm); err != nil {
			log.Fatal(err)
		}
	},
}
