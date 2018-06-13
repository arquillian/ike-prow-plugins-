package wip

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/arquillian/ike-prow-plugins/pkg/command"
	"github.com/arquillian/ike-prow-plugins/pkg/github"
	"github.com/arquillian/ike-prow-plugins/pkg/github/client"
	"github.com/arquillian/ike-prow-plugins/pkg/github/service"
	"github.com/arquillian/ike-prow-plugins/pkg/log"
	"github.com/arquillian/ike-prow-plugins/pkg/scm"
	"github.com/arquillian/ike-prow-plugins/pkg/utils"
	gogh "github.com/google/go-github/github"
)

const (
	// ProwPluginName is an external prow plugin name used to register this service
	ProwPluginName = "work-in-progress"

	// InProgressMessage is a message used in GH Status as description when the PR is in progress
	InProgressMessage = "PR is in progress and can't be merged yet. You might want to wait with review as well"
	// InProgressDetailsPageName is a name of a documentation page that contains additional status details for InProgressMessage
	InProgressDetailsPageName = "wip-failed"

	// ReadyForReviewMessage is a message used in GH Status as description when the PR is ready for review and merge
	ReadyForReviewMessage = "PR is ready for review and merge"
	// ReadyForReviewDetailsPageName is a name of a documentation page that contains additional status details for ReadyForReviewMessage
	ReadyForReviewDetailsPageName = "wip-success"
)

// GitHubWIPPRHandler handles PR events and updates status of the PR based on work-in-progress indicator
type GitHubWIPPRHandler struct {
	Client  ghclient.Client
	BotName string
}

var (
	handledCommentActions = []string{"created"}
	handledPrActions      = []string{"opened", "reopened", "edited", "synchronize", "labeled", "unlabeled"}
	defaultPrefixes       = []string{"WIP", "DO NOT MERGE", "DON'T MERGE", "WORK-IN-PROGRESS"}
)

// HandleEvent is an entry point for the plugin logic. This method is invoked by the Server when
// events are dispatched from the /hook service
func (gh *GitHubWIPPRHandler) HandleEvent(log log.Logger, eventType github.EventType, payload []byte) error {
	switch eventType {
	case github.PullRequest:
		var event gogh.PullRequestEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Errorf("Failed while parsing '%q' event with payload: %q. Cause: %q", github.PullRequest, event, err)
			return err
		}

		if err := gh.handlePrEvent(log, &event); err != nil {
			log.Errorf("Error handling '%q' event with payload %q. Cause: %q", github.PullRequest, event, err)
			return err
		}

	case github.IssueComment:
		var event gogh.IssueCommentEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Errorf("Failed while parsing '%q' event with payload: %q. Cause: %q", github.IssueComment, event, err)
			return err
		}

		if err := gh.handlePrComment(log, &event); err != nil {
			log.Errorf("Error handling '%q' event with payload %q. Cause: %q", github.IssueComment, event, err)
			return err
		}

	default:
		log.Warnf("received an event of type %q but didn't ask for it", eventType)
	}

	return nil
}

func (gh *GitHubWIPPRHandler) handlePrEvent(log log.Logger, event *gogh.PullRequestEvent) error {
	if !utils.Contains(handledPrActions, *event.Action) {
		return nil
	}

	switch *event.Action {
	case github.ActionLabeled:
		return gh.checkLabelAndSetStatus(log, event.PullRequest)

	case github.ActionUnlabeled:
		return gh.updateTitleAndResetStatus(log, event.PullRequest)

	default:
		return gh.checkTitleAndSetStatus(log, event.PullRequest)
	}
}

func (gh *GitHubWIPPRHandler) handlePrComment(log log.Logger, comment *gogh.IssueCommentEvent) error {
	if !utils.Contains(handledCommentActions, *comment.Action) {
		return nil
	}

	prLoader := ghservice.NewPullRequestLazyLoaderFromComment(gh.Client, comment)
	userPerm := command.NewPermissionService(gh.Client, *comment.Sender.Login, prLoader)

	cmdHandler := command.CommentCmdHandler{Client: gh.Client}
	cmdHandler.Register(&command.RunCmd{
		PluginName:            ProwPluginName,
		UserPermissionService: userPerm,
		WhenAddedOrEdited: func() error {
			pullRequest, err := prLoader.Load()
			if err != nil {
				return err
			}

			return gh.checkTitleAndSetStatus(log, pullRequest)

		}})

	err := cmdHandler.Handle(log, comment)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (gh *GitHubWIPPRHandler) checkTitleAndSetStatus(log log.Logger, pullRequest *gogh.PullRequest) error {
	change := ghservice.NewRepositoryChangeForPR(pullRequest)
	statusService := gh.getStatusService(log, change)
	configuration := LoadConfiguration(log, change)

	labelExists := gh.hasWorkInProgressLabel(pullRequest.Labels, configuration.Label)
	if ok, _ := gh.HasWorkInProgressPrefix(*pullRequest.Title, configuration); ok {
		if !labelExists {
			if err := gh.Client.AddPullRequestLabel(change, *pullRequest.Number, strings.Fields(configuration.Label)); err != nil {
				log.Errorf("failed to add label on PR [%q]. cause: %s", *pullRequest, err)
			}
		}
		return statusService.Failure(InProgressMessage, InProgressDetailsPageName)
	}
	if labelExists {
		if err := gh.Client.RemovePullRequestLabel(change, *pullRequest.Number, configuration.Label); err != nil {
			log.Errorf("failed to remove label on PR [%q]. cause: %s", *pullRequest, err)
		}
	}
	return statusService.Success(ReadyForReviewMessage, ReadyForReviewDetailsPageName)

}

func (gh *GitHubWIPPRHandler) updateTitleAndResetStatus(log log.Logger, pullRequest *gogh.PullRequest) error {
	change := ghservice.NewRepositoryChangeForPR(pullRequest)
	statusService := gh.getStatusService(log, change)
	configuration := LoadConfiguration(log, change)

	labelExists := gh.hasWorkInProgressLabel(pullRequest.Labels, configuration.Label)
	if !labelExists {
		if ok, prefix := gh.HasWorkInProgressPrefix(*pullRequest.Title, configuration); ok {
			*pullRequest.Title = strings.TrimPrefix(*pullRequest.Title, prefix)
			err := gh.Client.EditPullRequest(pullRequest)
			if err != nil {
				return fmt.Errorf("failed to update PR title [%q]. cause: %s", *pullRequest, err)
			}
		}
		return statusService.Success(ReadyForReviewMessage, ReadyForReviewDetailsPageName)
	}
	return statusService.Failure(InProgressMessage, InProgressDetailsPageName)
}

func (gh *GitHubWIPPRHandler) checkLabelAndSetStatus(log log.Logger, pullRequest *gogh.PullRequest) error {
	change := ghservice.NewRepositoryChangeForPR(pullRequest)
	statusService := gh.getStatusService(log, change)
	configuration := LoadConfiguration(log, change)

	labelExists := gh.hasWorkInProgressLabel(pullRequest.Labels, configuration.Label)
	if labelExists {
		return statusService.Failure(InProgressMessage, InProgressDetailsPageName)
	}
	return statusService.Success(ReadyForReviewMessage, ReadyForReviewDetailsPageName)
}

func (gh *GitHubWIPPRHandler) hasWorkInProgressLabel(labels []*gogh.Label, wipLabel string) bool {
	for _, label := range labels {
		if label.GetName() == wipLabel {
			return true
		}
	}
	return false
}

// HasWorkInProgressPrefix checks if title is marked as Work In Progress
func (gh *GitHubWIPPRHandler) HasWorkInProgressPrefix(title string, config PluginConfiguration) (bool, string) {
	prefixes := defaultPrefixes
	if len(config.Prefix) != 0 {
		if config.Combine {
			prefixes = append(prefixes, config.Prefix...)
		} else {
			prefixes = config.Prefix
		}
	}
	return gh.hasPrefix(strings.ToLower(title), prefixes)
}

func (gh *GitHubWIPPRHandler) hasPrefix(title string, prefixes []string) (bool, string) {
	for _, prefix := range prefixes {
		pattern := `(?mi)^(\[|\()?` + prefix + `(\]|\))?(:| )+`
		if match, _ := regexp.MatchString(pattern, title); match {
			return true, prefix
		}
	}
	return false, ""
}

func (gh *GitHubWIPPRHandler) getStatusService(log log.Logger, change scm.RepositoryChange) scm.StatusService {
	statusContext := github.StatusContext{BotName: gh.BotName, PluginName: ProwPluginName}
	return ghservice.NewStatusService(gh.Client, log, change, statusContext)
}
