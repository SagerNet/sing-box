package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/cidertool/asc-go/asc"
)

func main() {
	ctx := context.Background()
	switch os.Args[1] {
	case "next_macos_project_version":
		err := fetchMacOSVersion(ctx)
		if err != nil {
			log.Fatal(err)
		}
	case "publish_testflight":
		err := publishTestflight(ctx)
		if err != nil {
			log.Fatal(err)
		}
	case "cancel_app_store":
		err := cancelAppStore(ctx, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
	case "prepare_app_store":
		err := prepareAppStore(ctx)
		if err != nil {
			log.Fatal(err)
		}
	case "publish_app_store":
		err := publishAppStore(ctx)
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

func createClient() *Client {
	privateKey, err := os.ReadFile(os.Getenv("ASC_KEY_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	tokenConfig, err := asc.NewTokenConfig(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_KEY_ISSUER_ID"), time.Minute, privateKey)
	if err != nil {
		log.Fatal(err)
	}
	return &Client{asc.NewClient(tokenConfig.Client())}
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

func cancelAppStore(ctx context.Context, platform string) error {
	switch platform {
	case "ios":
		platform = string(asc.PlatformIOS)
	case "macos":
		platform = string(asc.PlatformMACOS)
	case "tvos":
		platform = string(asc.PlatformTVOS)
	}
	tag, err := build_shared.ReadTag()
	if err != nil {
		return err
	}
	client := createClient()
	log.Info(platform, " list versions")
	versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
		FilterPlatform: []string{string(platform)},
	})
	if err != nil {
		return err
	}
	version := common.Find(versions.Data, func(it asc.AppStoreVersion) bool {
		return *it.Attributes.VersionString == tag
	})
	if version.ID == "" {
		return nil
	}
	log.Info(string(platform), " ", tag, " get submission")
	submission, response, err := client.Submission.GetAppStoreVersionSubmissionForAppStoreVersion(ctx, version.ID, nil)
	if response != nil && response.StatusCode == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	log.Info(platform, " ", tag, " delete submission")
	_, err = client.Submission.DeleteSubmission(ctx, submission.Data.ID)
	if err != nil {
		return err
	}
	return nil
}

func prepareAppStore(ctx context.Context) error {
	tag, err := build_shared.ReadTag()
	if err != nil {
		return err
	}
	client := createClient()
	for _, platform := range []asc.Platform{
		asc.PlatformIOS,
		asc.PlatformMACOS,
		asc.PlatformTVOS,
	} {
		log.Info(string(platform), " list versions")
		versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
			FilterPlatform: []string{string(platform)},
		})
		if err != nil {
			return err
		}
		version := common.Find(versions.Data, func(it asc.AppStoreVersion) bool {
			return *it.Attributes.VersionString == tag
		})
		log.Info(string(platform), " ", tag, " list builds")
		builds, _, err := client.Builds.ListBuilds(ctx, &asc.ListBuildsQuery{
			FilterApp:                       []string{appID},
			FilterPreReleaseVersionPlatform: []string{string(platform)},
		})
		if err != nil {
			return err
		}
		if len(builds.Data) == 0 {
			log.Fatal(platform, " ", tag, " no build found")
		}
		buildID := common.Ptr(builds.Data[0].ID)
		if version.ID == "" {
			log.Info(string(platform), " ", tag, " create version")
			newVersion, _, err := client.Apps.CreateAppStoreVersion(ctx, asc.AppStoreVersionCreateRequestAttributes{
				Platform:      platform,
				VersionString: tag,
			}, appID, buildID)
			if err != nil {
				return err
			}
			version = newVersion.Data

		} else {
			log.Info(string(platform), " ", tag, " check build")
			currentBuild, response, err := client.Apps.GetBuildIDForAppStoreVersion(ctx, version.ID)
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK || currentBuild.Data.ID != *buildID {
				switch *version.Attributes.AppStoreState {
				case asc.AppStoreVersionStatePrepareForSubmission,
					asc.AppStoreVersionStateRejected,
					asc.AppStoreVersionStateDeveloperRejected:
				case asc.AppStoreVersionStateWaitingForReview,
					asc.AppStoreVersionStateInReview,
					asc.AppStoreVersionStatePendingDeveloperRelease:
					submission, _, err := client.Submission.GetAppStoreVersionSubmissionForAppStoreVersion(ctx, version.ID, nil)
					if err != nil {
						return err
					}
					if submission != nil {
						log.Info(string(platform), " ", tag, " delete submission")
						_, err = client.Submission.DeleteSubmission(ctx, submission.Data.ID)
						if err != nil {
							return err
						}
						time.Sleep(5 * time.Second)
					}
				default:
					log.Fatal(string(platform), " ", tag, " unknown state ", string(*version.Attributes.AppStoreState))
				}
				log.Info(string(platform), " ", tag, " update build")
				response, err = client.UpdateBuildForAppStoreVersion(ctx, version.ID, buildID)
				if err != nil {
					return err
				}
				if response.StatusCode != http.StatusNoContent {
					response.Write(os.Stderr)
					log.Fatal(string(platform), " ", tag, " unexpected response: ", response.Status)
				}
			} else {
				switch *version.Attributes.AppStoreState {
				case asc.AppStoreVersionStatePrepareForSubmission,
					asc.AppStoreVersionStateRejected,
					asc.AppStoreVersionStateDeveloperRejected:
				case asc.AppStoreVersionStateWaitingForReview,
					asc.AppStoreVersionStateInReview,
					asc.AppStoreVersionStatePendingDeveloperRelease:
					continue
				default:
					log.Fatal(string(platform), " ", tag, " unknown state ", string(*version.Attributes.AppStoreState))
				}
			}
		}
		log.Info(string(platform), " ", tag, " list localization")
		localizations, _, err := client.Apps.ListLocalizationsForAppStoreVersion(ctx, version.ID, nil)
		if err != nil {
			return err
		}
		localization := common.Find(localizations.Data, func(it asc.AppStoreVersionLocalization) bool {
			return *it.Attributes.Locale == "en-US"
		})
		if localization.ID == "" {
			log.Info(string(platform), " ", tag, " no en-US localization found")
		}
		if localization.Attributes.WhatsNew == nil && *localization.Attributes.WhatsNew == "" {
			log.Info(string(platform), " ", tag, " update localization")
			_, _, err = client.Apps.UpdateAppStoreVersionLocalization(ctx, localization.ID, &asc.AppStoreVersionLocalizationUpdateRequestAttributes{
				PromotionalText: common.Ptr("Yet another distribution for sing-box, the universal proxy platform."),
				WhatsNew:        common.Ptr(F.ToString("sing-box ", tag, ": Fixes and improvements.")),
			})
			if err != nil {
				return err
			}
		}
		log.Info(string(platform), " ", tag, " create submission")
	fixSubmit:
		for {
			_, response, err := client.Submission.CreateSubmission(ctx, version.ID)
			if err != nil {
				switch response.StatusCode {
				case http.StatusInternalServerError:
					continue
				default:
					response.Write(os.Stderr)
					log.Info(string(platform), " ", tag, " unexpected response: ", response.Status)
				}
			}
			switch response.StatusCode {
			case http.StatusCreated:
				break fixSubmit
			default:
				response.Write(os.Stderr)
				log.Info(string(platform), " ", tag, " unexpected response: ", response.Status)
			}
		}
	}
	return nil
}

func publishAppStore(ctx context.Context) error {
	tag, err := build_shared.ReadTag()
	if err != nil {
		return err
	}
	client := createClient()
	for _, platform := range []asc.Platform{
		asc.PlatformIOS,
		asc.PlatformMACOS,
		asc.PlatformTVOS,
	} {
		log.Info(string(platform), " list versions")
		versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
			FilterPlatform: []string{string(platform)},
		})
		if err != nil {
			return err
		}
		version := common.Find(versions.Data, func(it asc.AppStoreVersion) bool {
			return *it.Attributes.VersionString == tag
		})
		switch *version.Attributes.AppStoreState {
		case asc.AppStoreVersionStatePrepareForSubmission, asc.AppStoreVersionStateDeveloperRejected:
			log.Fatal(string(platform), " ", tag, " not submitted")
		case asc.AppStoreVersionStateWaitingForReview,
			asc.AppStoreVersionStateInReview:
			log.Warn(string(platform), " ", tag, " waiting for review")
			continue
		case asc.AppStoreVersionStatePendingDeveloperRelease:
		default:
			log.Fatal(string(platform), " ", tag, " unknown state ", string(*version.Attributes.AppStoreState))
		}
		_, _, err = client.Publishing.CreatePhasedRelease(ctx, common.Ptr(asc.PhasedReleaseStateComplete), version.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
