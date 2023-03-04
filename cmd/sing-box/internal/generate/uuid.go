package generate

import (
	"os"

	"github.com/sagernet/sing-box/log"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"
)

var CommandGenerateUUID = &cobra.Command{
	Use:   "uuid",
	Short: "Generate a UUID",
	Run: func(cmd *cobra.Command, args []string) {
		var (
			newUUID uuid.UUID
			err     error
		)

		if input == "" {
			newUUID, err = uuid.NewV4()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			newUUID = uuid.NewV5(uuid.Nil, input)
		}

		os.Stdout.WriteString(newUUID.String() + "\n")
	},
}

func init() {
	CommandGenerateUUID.Flags().StringVarP(&input, "input", "i", "", "generate UUID v5 from specified string")
}
