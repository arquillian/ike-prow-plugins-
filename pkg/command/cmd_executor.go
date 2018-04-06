package command

import (
	"strings"

	"github.com/arquillian/ike-prow-plugins/pkg/github"
	"github.com/arquillian/ike-prow-plugins/pkg/utils"
	gogh "github.com/google/go-github/github"
	"github.com/arquillian/ike-prow-plugins/pkg/log"
)

// DoFunction is used for performing operations related to command actions
type DoFunction func() error
type doFunctionExecutor func(client *github.Client, log log.Logger, prComment *gogh.IssueCommentEvent) error

// CmdExecutor takes care of executing a command triggered by IssueCommentEvent.
// The execution is set by specifying actions/events and with given restrictions the command should be triggered for.
type CmdExecutor struct {
	Command    string
	QuiteMode  bool
	doExecutor []doFunctionExecutor
}

// RestrictionSetter keeps information about set actions the command should be triggered for and opens an API to provide
// permission restrictions
type RestrictionSetter struct {
	commandExecutor *CmdExecutor
	actions         []commentAction
}

// DoFunctionProvider keeps all allowed actions and permission checks and opens an API to provide a DoFunction implementation
type DoFunctionProvider struct {
	commandExecutor  *CmdExecutor
	actions          []commentAction
	permissionChecks []PermissionCheck
}

type commentAction struct {
	actions          []string
	operationMsgName string
}

// Deleted represents comment deletion
var Deleted = commentAction{actions: []string{"deleted"}, operationMsgName: "deleted"}

// Triggered represents comment editions and creation
var Triggered = commentAction{actions: []string{"edited", "created"}, operationMsgName: "used"}

func (a *commentAction) isMatching(prComment *gogh.IssueCommentEvent) bool {
	return utils.Contains(a.actions, *prComment.Action)
}

// When takes list of actions the command should be triggered for
func (e *CmdExecutor) When(actions ...commentAction) *RestrictionSetter {
	return &RestrictionSetter{commandExecutor: e, actions: actions}
}

// By takes a list of permission checks the command should be restricted by
func (s *RestrictionSetter) By(permissionChecks ...PermissionCheck) *DoFunctionProvider {
	return &DoFunctionProvider{commandExecutor: s.commandExecutor, actions: s.actions, permissionChecks: permissionChecks}
}

// ThenDo take a DoFunction that performs the required operations (when all checks are fulfilled)
func (p *DoFunctionProvider) ThenDo(doFunction DoFunction) {
	doExecutor := func(client *github.Client, log log.Logger, prComment *gogh.IssueCommentEvent) error {
		matchingAction := p.getMatchingAction(prComment)
		if matchingAction == nil {
			return nil
		}

		status, err := AllOf(p.permissionChecks...)()
		if status.UserIsApproved && err == nil {
			return doFunction()
		}
		message := status.constructMessage(matchingAction.operationMsgName, p.commandExecutor.Command)
		log.Warn(message)
		if err == nil && !p.commandExecutor.QuiteMode {
			commentService := github.NewCommentService(client, prComment)
			return commentService.AddComment(&message)
		}
		return err
	}

	p.commandExecutor.doExecutor = append(p.commandExecutor.doExecutor, doExecutor)
}

func (p *DoFunctionProvider) getMatchingAction(prComment *gogh.IssueCommentEvent) *commentAction {
	for _, action := range p.actions {
		if action.isMatching(prComment) {
			return &action
		}
	}
	return nil
}

// Execute triggers the given DoFunctions (when all checks are fulfilled) for the given pr comment
func (e *CmdExecutor) Execute(client *github.Client, log log.Logger, prComment *gogh.IssueCommentEvent) error {
	if e.Command != strings.TrimSpace(*prComment.Comment.Body) {
		return nil
	}
	for _, doExecutor := range e.doExecutor {
		err := doExecutor(client, log, prComment)
		if err != nil {
			return err
		}
	}
	return nil
}
