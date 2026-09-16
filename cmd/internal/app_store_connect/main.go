package main

import (
	"context"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/asc-go/asc"
	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/common/badversion"
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
	case "list_builds":
		if len(os.Args) < 3 {
			log.Fatal("platform required: ios, macos, or tvos")
		}
		err := listBuilds(ctx, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
	case "remove_testflight":
		if len(os.Args) < 5 {
			log.Fatal("usage: remove_testflight <platform> <version> <build>")
		}
		err := removeTestflight(ctx, os.Args[2], os.Args[3], os.Args[4])
		if err != nil {
			log.Fatal(err)
		}
	case "cancel_app_store":
		if len(os.Args) < 3 {
			log.Fatal("platform required: ios, macos, or tvos")
		}
		err := cancelAppStore(ctx, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
	case "submit_app_store":
		err := submitAppStore(ctx, os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}
	case "publish_app_store":
		err := publishAppStore(ctx, os.Args[2:])
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

	applePath               = "../sing-box-for-apple"
	appStorePromotionalText = "The universal proxy platform."
	appStoreWhatsNew        = "Fixes and improvements"
	buildPollInterval       = 15 * time.Second
)

var (
	appStorePlatforms   = []string{"ios", "tvos"}
	platformDirectories = map[asc.Platform]string{
		asc.PlatformIOS:   "SFI",
		asc.PlatformMACOS: "SFM",
		asc.PlatformTVOS:  "SFT",
	}
)

func parsePlatform(name string) (asc.Platform, error) {
	switch name {
	case "ios":
		return asc.PlatformIOS, nil
	case "macos":
		return asc.PlatformMACOS, nil
	case "tvos":
		return asc.PlatformTVOS, nil
	default:
		return "", E.New("unknown platform: ", name)
	}
}

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
	platform, err := parsePlatform(platformName)
	if err != nil {
		return err
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

func listBuilds(ctx context.Context, platformName string) error {
	platform, err := parsePlatform(platformName)
	if err != nil {
		return err
	}
	client := createClient(time.Minute)
	buildIDsResponse, _, err := client.TestFlight.ListBuildIDsForBetaGroup(ctx, groupID, nil)
	if err != nil {
		return err
	}
	betaBuildIDs := common.Map(buildIDsResponse.Data, func(it asc.RelationshipData) string {
		return it.ID
	})
	builds, _, err := client.Builds.ListBuilds(ctx, &asc.ListBuildsQuery{
		FilterApp:                       []string{appID},
		FilterPreReleaseVersionPlatform: []string{string(platform)},
		Include:                         []string{"preReleaseVersion"},
		Sort:                            []string{"-uploadedDate"},
		Limit:                           10,
	})
	if err != nil {
		return err
	}
	for _, build := range builds.Data {
		var betaGroup string
		if slices.Contains(betaBuildIDs, build.ID) {
			betaGroup = " testflight"
		}
		var versionString string
		if build.Relationships != nil && build.Relationships.PreReleaseVersion != nil && build.Relationships.PreReleaseVersion.Data != nil {
			for _, included := range builds.Included {
				preReleaseVersion := included.PrereleaseVersion()
				if preReleaseVersion != nil && preReleaseVersion.ID == build.Relationships.PreReleaseVersion.Data.ID {
					versionString = *preReleaseVersion.Attributes.Version
				}
			}
		}
		os.Stdout.WriteString(F.ToString(versionString, " (", *build.Attributes.Version, ") ", *build.Attributes.ProcessingState, " ", build.Attributes.UploadedDate.Time.Format(time.RFC3339), betaGroup, "\n"))
	}
	return nil
}

func publishTestflight(ctx context.Context) error {
	if len(os.Args) < 3 {
		return E.New("platform required: ios, macos, or tvos")
	}
	platform, err := parsePlatform(os.Args[2])
	if err != nil {
		return err
	}

	tagVersion, err := build_shared.ReadTagVersion()
	if err != nil {
		return err
	}
	tag := tagVersion.VersionString()
	testFlightVersion := build_shared.TestFlightVersion(tagVersion)
	projectVersion := os.Getenv(strings.ToUpper(os.Args[2]) + "_PROJECT_VERSION")

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

	query := &asc.ListBuildsQuery{
		FilterApp:                       []string{appID},
		FilterPreReleaseVersionPlatform: []string{string(platform)},
		FilterPreReleaseVersionVersion:  []string{testFlightVersion},
		Sort:                            []string{"-uploadedDate"},
		Limit:                           1,
	}
	if projectVersion != "" {
		query.FilterVersion = []string{projectVersion}
	}
	log.Info(string(platform), " ", testFlightVersion, " (", projectVersion, ") list builds")
	for {
		builds, _, err := client.Builds.ListBuilds(ctx, query)
		if err != nil {
			return err
		}
		if len(builds.Data) == 0 {
			log.Info(string(platform), " ", testFlightVersion, " waiting for build upload")
			time.Sleep(buildPollInterval)
			continue
		}
		build := builds.Data[0]
		log.Info(string(platform), " ", testFlightVersion, " found build: ", build.ID, " (", *build.Attributes.Version, ")")
		if projectVersion == "" && common.Contains(buildIDs, build.ID) {
			log.Info(string(platform), " ", testFlightVersion, " build ", *build.Attributes.Version, " already published, waiting for new upload")
			time.Sleep(buildPollInterval)
			continue
		}
		if *build.Attributes.ProcessingState != "VALID" {
			log.Info(string(platform), " ", testFlightVersion, " waiting for process: ", *build.Attributes.ProcessingState)
			time.Sleep(buildPollInterval)
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
			time.Sleep(buildPollInterval)
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

func removeTestflight(ctx context.Context, platformName string, versionString string, buildNumber string) error {
	platform, err := parsePlatform(platformName)
	if err != nil {
		return err
	}
	client := createClient(time.Minute)
	builds, _, err := client.Builds.ListBuilds(ctx, &asc.ListBuildsQuery{
		FilterApp:                       []string{appID},
		FilterPreReleaseVersionPlatform: []string{string(platform)},
		FilterPreReleaseVersionVersion:  []string{versionString},
		FilterVersion:                   []string{buildNumber},
	})
	if err != nil {
		return err
	}
	if len(builds.Data) != 1 {
		return E.New(string(platform), " ", versionString, " (", buildNumber, ") matched ", len(builds.Data), " builds")
	}
	build := builds.Data[0]
	log.Info(string(platform), " ", versionString, " (", buildNumber, ") remove from beta group")
	_, err = client.Builds.RemoveAccessForBetaGroupsFromBuild(ctx, build.ID, []string{groupID})
	if err != nil {
		return err
	}
	return nil
}

func cancelAppStore(ctx context.Context, platformName string) error {
	platform, err := parsePlatform(platformName)
	if err != nil {
		return err
	}
	tag, err := build_shared.ReadTag()
	if err != nil {
		return err
	}
	client := createClient(time.Minute)
	log.Info(string(platform), " list versions")
	versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
		FilterPlatform:      []string{string(platform)},
		FilterVersionString: []string{tag},
	})
	if err != nil {
		return err
	}
	if len(versions.Data) == 0 {
		return nil
	}
	versionID := versions.Data[0].ID
	log.Info(string(platform), " ", tag, " list review submissions")
	submissions, _, err := client.Submission.ListReviewSubmissions(ctx, &asc.ListReviewSubmissionsQuery{
		FilterApp:      []string{appID},
		FilterPlatform: []string{string(platform)},
		FilterState: []string{
			string(asc.ReviewSubmissionStateReadyForReview),
			string(asc.ReviewSubmissionStateWaitingForReview),
			string(asc.ReviewSubmissionStateInReview),
		},
	})
	if err != nil {
		return err
	}
	for _, submission := range submissions.Data {
		containsVersion, err := reviewSubmissionContainsVersion(ctx, client, submission.ID, versionID)
		if err != nil {
			return err
		}
		if !containsVersion {
			continue
		}
		log.Info(string(platform), " ", tag, " cancel review submission ", submission.ID)
		_, _, err = client.Submission.UpdateReviewSubmission(ctx, submission.ID, nil, common.Ptr(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func reviewSubmissionContainsVersion(ctx context.Context, client *asc.Client, submissionID string, versionID string) (bool, error) {
	items, _, err := client.Submission.ListItemsForReviewSubmission(ctx, submissionID, nil)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(items.Data, func(it asc.ReviewSubmissionItem) bool {
		return it.Relationships != nil && it.Relationships.AppStoreVersion != nil &&
			it.Relationships.AppStoreVersion.Data != nil && it.Relationships.AppStoreVersion.Data.ID == versionID
	}), nil
}

func submitAppStore(ctx context.Context, platformNames []string) error {
	if len(platformNames) == 0 {
		platformNames = appStorePlatforms
	}
	client := createClient(20 * time.Minute)
	for _, platformName := range platformNames {
		platform, err := parsePlatform(platformName)
		if err != nil {
			return err
		}
		versionString, err := build_shared.ReadAppleMarketingVersion(applePath, platformDirectories[platform])
		if err != nil {
			return err
		}
		err = submitAppStoreVersion(ctx, client, platform, versionString)
		if err != nil {
			return err
		}
	}
	return nil
}

func submitAppStoreVersion(ctx context.Context, client *asc.Client, platform asc.Platform, versionString string) error {
	log.Info(string(platform), " ", versionString, " list versions")
	versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
		FilterPlatform: []string{string(platform)},
	})
	if err != nil {
		return err
	}
	localVersion := badversion.Parse(versionString)
	var version asc.AppStoreVersion
	for _, remoteVersion := range versions.Data {
		remoteVersionString := *remoteVersion.Attributes.VersionString
		if remoteVersionString == versionString {
			version = remoteVersion
			continue
		}
		if !badversion.Parse(remoteVersionString).LessThan(localVersion) {
			return E.New(string(platform), " ", versionString, " is not newer than App Store version ", remoteVersionString)
		}
	}
	if version.ID != "" {
		state := *version.Attributes.AppStoreState
		switch state {
		case asc.AppStoreVersionStatePrepareForSubmission,
			asc.AppStoreVersionStateRejected,
			asc.AppStoreVersionStateDeveloperRejected,
			asc.AppStoreVersionStateMetadataRejected,
			asc.AppStoreVersionStateInvalidBinary:
			log.Info(string(platform), " ", versionString, " found version in state ", string(state))
		case asc.AppStoreVersionStateWaitingForReview, asc.AppStoreVersionStateInReview:
			log.Info(string(platform), " ", versionString, " already submitted: ", string(state))
			return nil
		case asc.AppStoreVersionStatePendingDeveloperRelease:
			log.Info(string(platform), " ", versionString, " approved, run publish_app_store to release")
			return nil
		default:
			return E.New(string(platform), " ", versionString, " already released: ", string(state))
		}
	} else {
		log.Info(string(platform), " ", versionString, " create version")
		created, _, err := client.Apps.CreateAppStoreVersion(ctx, asc.AppStoreVersionCreateRequestAttributes{
			Platform:      platform,
			ReleaseType:   common.Ptr("MANUAL"),
			VersionString: versionString,
		}, appID, nil)
		if err != nil {
			return err
		}
		version = created.Data
	}

	log.Info(string(platform), " ", versionString, " list localizations")
	localizations, _, err := client.Apps.ListLocalizationsForAppStoreVersion(ctx, version.ID, nil)
	if err != nil {
		return err
	}
	localization := common.Find(localizations.Data, func(it asc.AppStoreVersionLocalization) bool {
		return *it.Attributes.Locale == "en-US"
	})
	if localization.ID == "" {
		log.Info(string(platform), " ", versionString, " create en-US localization")
		_, _, err = client.Apps.CreateAppStoreVersionLocalization(ctx, asc.AppStoreVersionLocalizationCreateRequestAttributes{
			Locale:          "en-US",
			PromotionalText: common.Ptr(appStorePromotionalText),
			WhatsNew:        common.Ptr(appStoreWhatsNew),
		}, version.ID)
		if err != nil {
			return err
		}
	} else if localization.Attributes.PromotionalText == nil || *localization.Attributes.PromotionalText != appStorePromotionalText ||
		localization.Attributes.WhatsNew == nil || *localization.Attributes.WhatsNew != appStoreWhatsNew {
		log.Info(string(platform), " ", versionString, " update en-US localization")
		_, _, err = client.Apps.UpdateAppStoreVersionLocalization(ctx, localization.ID, &asc.AppStoreVersionLocalizationUpdateRequestAttributes{
			PromotionalText: common.Ptr(appStorePromotionalText),
			WhatsNew:        common.Ptr(appStoreWhatsNew),
		})
		if err != nil {
			return err
		}
	}

	var build asc.Build
	for {
		builds, _, err := client.Builds.ListBuilds(ctx, &asc.ListBuildsQuery{
			FilterApp:                       []string{appID},
			FilterPreReleaseVersionPlatform: []string{string(platform)},
			FilterPreReleaseVersionVersion:  []string{versionString},
			Sort:                            []string{"-uploadedDate"},
			Limit:                           1,
		})
		if err != nil {
			return err
		}
		if len(builds.Data) == 0 {
			log.Info(string(platform), " ", versionString, " waiting for build upload")
			time.Sleep(buildPollInterval)
			continue
		}
		build = builds.Data[0]
		processingState := *build.Attributes.ProcessingState
		if processingState == "VALID" {
			break
		}
		if processingState == "PROCESSING" {
			log.Info(string(platform), " ", versionString, " waiting for build ", *build.Attributes.Version, ": ", processingState)
			time.Sleep(buildPollInterval)
			continue
		}
		return E.New(string(platform), " ", versionString, " build ", *build.Attributes.Version, " ", processingState)
	}
	log.Info(string(platform), " ", versionString, " found build ", *build.Attributes.Version)

	currentBuild, response, err := client.Apps.GetBuildIDForAppStoreVersion(ctx, version.ID)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || currentBuild.Data.ID != build.ID {
		log.Info(string(platform), " ", versionString, " attach build ", *build.Attributes.Version)
		_, err = client.Apps.UpdateBuildForAppStoreVersion(ctx, version.ID, common.Ptr(build.ID))
		if err != nil {
			return err
		}
	}

	log.Info(string(platform), " ", versionString, " list review submissions")
	submissions, _, err := client.Submission.ListReviewSubmissions(ctx, &asc.ListReviewSubmissionsQuery{
		FilterApp:      []string{appID},
		FilterPlatform: []string{string(platform)},
		FilterState: []string{
			string(asc.ReviewSubmissionStateReadyForReview),
			string(asc.ReviewSubmissionStateWaitingForReview),
			string(asc.ReviewSubmissionStateInReview),
			string(asc.ReviewSubmissionStateUnresolvedIssues),
		},
	})
	if err != nil {
		return err
	}
	var (
		submission        asc.ReviewSubmission
		submissionHasItem bool
	)
	for _, remoteSubmission := range submissions.Data {
		containsVersion, err := reviewSubmissionContainsVersion(ctx, client, remoteSubmission.ID, version.ID)
		if err != nil {
			return err
		}
		state := *remoteSubmission.Attributes.State
		switch state {
		case asc.ReviewSubmissionStateReadyForReview:
			submission = remoteSubmission
			submissionHasItem = containsVersion
		case asc.ReviewSubmissionStateWaitingForReview, asc.ReviewSubmissionStateInReview:
			if containsVersion {
				log.Info(string(platform), " ", versionString, " already submitted: ", string(state))
				return nil
			}
			return E.New(string(platform), " review submission ", remoteSubmission.ID, " with another version is ", string(state))
		default:
			return E.New(string(platform), " review submission ", remoteSubmission.ID, " is ", string(state))
		}
	}
	if submission.ID == "" {
		log.Info(string(platform), " ", versionString, " create review submission")
		created, _, err := client.Submission.CreateReviewSubmission(ctx, appID, common.Ptr(platform))
		if err != nil {
			return err
		}
		submission = created.Data
	}
	if !submissionHasItem {
		log.Info(string(platform), " ", versionString, " add version to review submission ", submission.ID)
		_, _, err = client.Submission.CreateReviewSubmissionItemForAppStoreVersion(ctx, submission.ID, version.ID)
		if err != nil {
			return err
		}
	}
	log.Info(string(platform), " ", versionString, " submit review submission ", submission.ID)
	_, _, err = client.Submission.UpdateReviewSubmission(ctx, submission.ID, common.Ptr(true), nil)
	if err != nil {
		return err
	}
	return nil
}

func publishAppStore(ctx context.Context, platformNames []string) error {
	if len(platformNames) == 0 {
		platformNames = appStorePlatforms
	}
	client := createClient(time.Minute)
	for _, platformName := range platformNames {
		platform, err := parsePlatform(platformName)
		if err != nil {
			return err
		}
		versionString, err := build_shared.ReadAppleMarketingVersion(applePath, platformDirectories[platform])
		if err != nil {
			return err
		}
		log.Info(string(platform), " ", versionString, " list versions")
		versions, _, err := client.Apps.ListAppStoreVersionsForApp(ctx, appID, &asc.ListAppStoreVersionsQuery{
			FilterPlatform:      []string{string(platform)},
			FilterVersionString: []string{versionString},
		})
		if err != nil {
			return err
		}
		if len(versions.Data) == 0 {
			return E.New(string(platform), " ", versionString, " not found")
		}
		version := versions.Data[0]
		state := *version.Attributes.AppStoreState
		switch state {
		case asc.AppStoreVersionStatePrepareForSubmission, asc.AppStoreVersionStateDeveloperRejected:
			return E.New(string(platform), " ", versionString, " not submitted")
		case asc.AppStoreVersionStateWaitingForReview, asc.AppStoreVersionStateInReview:
			log.Warn(string(platform), " ", versionString, " waiting for review")
			continue
		case asc.AppStoreVersionStatePendingDeveloperRelease:
		default:
			return E.New(string(platform), " ", versionString, " unknown state ", string(state))
		}
		log.Info(string(platform), " ", versionString, " release")
		_, _, err = client.Publishing.CreatePhasedRelease(ctx, common.Ptr(asc.PhasedReleaseStateComplete), version.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
