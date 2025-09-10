package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/asc-go/asc"
	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
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

func createClient(expireDuration time.Duration) *asc.Client {
	privateKey, err := os.ReadFile(os.Getenv("ASC_KEY_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	tokenConfig, err := asc.NewTokenConfig(os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_KEY_ISSUER_ID"), expireDuration, privateKey)
	if err != nil {
		log.Fatal(err)
	}
	return asc.NewClient(tokenConfig.Client())
}

func fetchMacOSVersion(ctx context.Context) error {
	client := createClient(time.Minute)
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
	tagVersion, err := build_shared.ReadTagVersion()
	if err != nil {
		return err
	}
	tag := tagVersion.VersionString()
	client := createClient(20 * time.Minute)

	log.Info(tag, " list build IDs")
	buildIDsResponse, _, err := client.TestFlight.ListBuildIDsForBetaGroup(ctx, groupID, nil)
	if err != nil {
		return err
	}
	buildIDs := common.Map(buildIDsResponse.Data, func(it asc.RelationshipData) string {
		return it.ID
	})
	var platforms []asc.Platform
	if len(os.Args) == 3 {
		switch os.Args[2] {
		case "ios":
			platforms = []asc.Platform{asc.PlatformIOS}
		case "macos":
			platforms = []asc.Platform{asc.PlatformMACOS}
		case "tvos":
			platforms = []asc.Platform{asc.PlatformTVOS}
		default:
			return E.New("unknown platform: ", os.Args[2])
		}
	} else {
		platforms = []asc.Platform{
			asc.PlatformIOS,
			asc.PlatformMACOS,
			asc.PlatformTVOS,
		}
	}
	waitingForProcess := false
	for _, platform := range platforms {
		log.Info(string(platform), " list builds")
		for {
			builds, _, err := client.Builds.ListBuilds(ctx, &asc.ListBuildsQuery{
				FilterApp:                       []string{appID},
				FilterPreReleaseVersionPlatform: []string{string(platform)},
			})
			if err != nil {
				return err
			}
			build := builds.Data[0]
			if !waitingForProcess && (common.Contains(buildIDs, build.ID) || time.Since(build.Attributes.UploadedDate.Time) > 30*time.Minute) {
				log.Info(string(platform), " ", tag, " waiting for process")
				time.Sleep(15 * time.Second)
				continue
			}
			if *build.Attributes.ProcessingState != "VALID" {
				waitingForProcess = true
				log.Info(string(platform), " ", tag, " waiting for process: ", *build.Attributes.ProcessingState)
				time.Sleep(15 * time.Second)
				continue
			}
			log.Info(string(platform), " ", tag, " list localizations")
			localizations, _, err := client.TestFlight.ListBetaBuildLocalizationsForBuild(ctx, build.ID, nil)
			if err != nil {
				return err
			}
			localization := common.Find(localizations.Data, func(it asc.BetaBuildLocalization) bool {
				return *it.Attributes.Locale == "en-US"
			})
			if localization.ID == "" {
				log.Fatal(string(platform), " ", tag, " no en-US localization found")
			}
			if localization.Attributes == nil || localization.Attributes.WhatsNew == nil || *localization.Attributes.WhatsNew == "" {
				log.Info(string(platform), " ", tag, " update localization")
				_, _, err = client.TestFlight.UpdateBetaBuildLocalization(ctx, localization.ID, common.Ptr(
					F.ToString("sing-box ", tagVersion.String()),
				))
				if err != nil {
					return err
				}
			}
			log.Info(string(platform), " ", tag, " publish")
			response, err := client.TestFlight.AddBuildsToBetaGroup(ctx, groupID, []string{build.ID})
			if response != nil && (response.StatusCode == http.StatusUnprocessableEntity || response.StatusCode == http.StatusNotFound) {
				log.Info("waiting for process")
				time.Sleep(15 * time.Second)
				continue
			} else if err != nil {
				return err
			}
			log.Info(string(platform), " ", tag, " list submissions")
			betaSubmissions, _, err := client.TestFlight.ListBetaAppReviewSubmissions(ctx, &asc.ListBetaAppReviewSubmissionsQuery{
				FilterBuild: []string{build.ID},
			})
			if err != nil {
				return err
			}
			if len(betaSubmissions.Data) == 0 {
				log.Info(string(platform), " ", tag, " create submission")
				_, _, err = client.TestFlight.CreateBetaAppReviewSubmission(ctx, build.ID)
				if err != nil {
					if strings.Contains(err.Error(), "ANOTHER_BUILD_IN_REVIEW") {
						log.Error(err)
						break
					}
					return err
				}
			}
			break
		}
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
	client := createClient(time.Minute)
	for {
		log.Info(platform, " list versions")
		versions, response, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
			FilterPlatform: []string{string(platform)},
		})
		if isRetryable(response) {
			continue
		} else if err != nil {
			return err
		}
		version := common.Find(versions.Data, func(it asc.AppStoreVersion) bool {
			return *it.Attributes.VersionString == tag
		})
		if version.ID == "" {
			return nil
		}
		log.Info(platform, " ", tag, " get submission")
		submission, response, err := client.Submission.GetAppStoreVersionSubmissionForAppStoreVersion(ctx, version.ID, nil)
		if response != nil && response.StatusCode == http.StatusNotFound {
			return nil
		}
		if isRetryable(response) {
			continue
		} else if err != nil {
			return err
		}
		log.Info(platform, " ", tag, " delete submission")
		_, err = client.Submission.DeleteSubmission(ctx, submission.Data.ID)
		if err != nil {
			return err
		}
		return nil
	}
}

func prepareAppStore(ctx context.Context) error {
	tag, err := build_shared.ReadTag()
	if err != nil {
		return err
	}
	client := createClient(time.Minute)
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
				response, err = client.Apps.UpdateBuildForAppStoreVersion(ctx, version.ID, buildID)
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
		if localization.Attributes == nil || localization.Attributes.WhatsNew == nil || *localization.Attributes.WhatsNew == "" {
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
					return err
				}
			}
			switch response.StatusCode {
			case http.StatusCreated:
				break fixSubmit
			default:
				return err
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
	client := createClient(time.Minute)
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

func isRetryable(response *asc.Response) bool {
	if response == nil {
		return false
	}
	switch response.StatusCode {
	case http.StatusInternalServerError, http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}
