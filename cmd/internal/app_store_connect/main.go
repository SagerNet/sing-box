package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/cidertool/asc-go/asc"
)

func main() {
	switch os.Args[1] {
	case "publish_testflight":
		err := publishTestflight(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	case "next_macos_project_version":
		err := fetchMacOSVersion(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown action: ", os.Args[1])
	}
}

const (
	appID   = "6673731168"
	groupID = "5c5f3b78-b7a0-40c0-bcad-e6ef87bbefda"
)

func createClient() *asc.Client {
	privateKey, err := os.ReadFile(os.Getenv("ASC_KEY_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	tokenConfig, err := asc.NewTokenConfig(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_KEY_ISSUER_ID"), time.Minute, privateKey)
	if err != nil {
		log.Fatal(err)
	}
	return asc.NewClient(tokenConfig.Client())
}

func publishTestflight(ctx context.Context) error {
	client := createClient()
	var buildsToPublish []asc.Build
	for _, platform := range []string{
		"IOS",
		"MAC_OS",
		"TV_OS",
	} {
		builds, _, err := client.Builds.ListBuilds(ctx, &asc.ListBuildsQuery{
			FilterApp:                       []string{appID},
			FilterPreReleaseVersionPlatform: []string{platform},
		})
		if err != nil {
			return err
		}
		buildsToPublish = append(buildsToPublish, builds.Data[0])
	}
	_, err := client.TestFlight.AddBuildsToBetaGroup(ctx, groupID, common.Map(buildsToPublish, func(it asc.Build) string {
		return it.ID
	}))
	if err != nil {
		return err
	}
	return nil
}

func fetchMacOSVersion(ctx context.Context) error {
	client := createClient()
	versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
		FilterPlatform: []string{"MAC_OS"},
	})
	if err != nil {
		return err
	}
	var versionID string
findVersion:
	for _, version := range versions.Data {
		switch *version.Attributes.AppStoreState {
		case asc.AppStoreVersionStateReadyForSale,
			asc.AppStoreVersionStatePendingDeveloperRelease:
			versionID = version.ID
			break findVersion
		}
	}
	if versionID == "" {
		return E.New("no version found")
	}
	latestBuild, _, err := client.Builds.GetBuildForAppStoreVersion(ctx, versionID, &asc.GetBuildForAppStoreVersionQuery{})
	if err != nil {
		return err
	}
	versionInt, err := strconv.Atoi(*latestBuild.Data.Attributes.Version)
	if err != nil {
		return E.Cause(err, "parse version code")
	}
	os.Stdout.WriteString(F.ToString(versionInt+1, "\n"))
	return nil
}
