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
	case "next_project_version":
		if len(os.Args) < 3 {
			log.Fatal("platform required: ios, macos, or tvos")
		}
		err := fetchNextProjectVersion(ctx, os.Args[2])
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
	appID   = "6785326793"
	groupID = "39f9ebdc-05d4-421f-9595-dae71df227c4"
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

func fetchNextProjectVersion(ctx context.Context, platformName string) error {
	var platform asc.Platform
	switch platformName {
	case "ios":
		platform = asc.PlatformIOS
	case "macos":
		platform = asc.PlatformMACOS
	case "tvos":
		platform = asc.PlatformTVOS
	default:
		return E.New("unknown platform: ", platformName)
	}

	query := &asc.ListBuildsQuery{
		FilterApp:                       []string{appID},
		FilterPreReleaseVersionPlatform: []string{string(platform)},
		Limit:                           200,
	}
	if platform != asc.PlatformMACOS {
		tagVersion, err := build_shared.ReadTagVersion()
		if err != nil {
			return err
		}
		query.FilterPreReleaseVersionVersion = []string{build_shared.TestFlightVersion(tagVersion)}
	}

	client := createClient(time.Minute)
	builds, _, err := client.Builds.ListBuilds(ctx, query)
	if err != nil {
		return err
	}
	nextProjectVersion := 1
	var projectVersion int
	for _, build := range builds.Data {
		projectVersion, err = strconv.Atoi(*build.Attributes.Version)
		if err != nil {
			return E.Cause(err, "parse version code")
		}
		if projectVersion >= nextProjectVersion {
			nextProjectVersion = projectVersion + 1
		}
	}
	os.Stdout.WriteString(F.ToString(nextProjectVersion, "\n"))
	return nil
}

func publishTestflight(ctx context.Context) error {
	if len(os.Args) < 3 {
		return E.New("platform required: ios, macos, or tvos")
	}
	var platform asc.Platform
	switch os.Args[2] {
	case "ios":
		platform = asc.PlatformIOS
	case "macos":
		platform = asc.PlatformMACOS
	case "tvos":
		platform = asc.PlatformTVOS
	default:
		return E.New("unknown platform: ", os.Args[2])
	}

	tagVersion, err := build_shared.ReadTagVersion()
	if err != nil {
		return err
	}
	tag := tagVersion.VersionString()

	releaseNotes := F.ToString("sing-box ", tagVersion.String())
	if len(os.Args) >= 4 {
		releaseNotes = strings.Join(os.Args[3:], " ")
	}

	client := createClient(20 * time.Minute)

	log.Info(tag, " list build IDs")
	buildIDsResponse, _, err := client.TestFlight.ListBuildIDsForBetaGroup(ctx, groupID, nil)
	if err != nil {
		return err
	}
	buildIDs := common.Map(buildIDsResponse.Data, func(it asc.RelationshipData) string {
		return it.ID
	})

	waitingForProcess := false
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
		log.Info(string(platform), " ", tag, " found build: ", build.ID, " (", *build.Attributes.Version, ")")
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
			_, _, err = client.TestFlight.UpdateBetaBuildLocalization(ctx, localization.ID, common.Ptr(releaseNotes))
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
			log.Fatal(string(platform), " ", tag, " no build found")
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
