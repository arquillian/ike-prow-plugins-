package plugin

import (
	"context"
	"flag"
	"net/url"
	"os/signal"
	"syscall"

	"golang.org/x/oauth2"

	"strconv"

	"github.com/arquillian/ike-prow-plugins/pkg/plugin/server"
	"github.com/arquillian/ike-prow-plugins/pkg/utils"
	"github.com/google/go-github/github"
	"k8s.io/test-infra/prow/pluginhelp/externalplugins"
	"k8s.io/test-infra/prow/plugins"

	"net/http"

	"github.com/arquillian/ike-prow-plugins/pkg/log"
	"github.com/sirupsen/logrus"
)

// nolint
var (
	port                = flag.Int("port", 8888, "Port to listen on.")
	dryRun              = flag.Bool("dry-run", true, "Dry run for testing. Uses API tokens but does not mutate.")
	pluginConfig        = flag.String("ike-plugins-config", "/etc/plugins/plugins", "Path to ike-plugins config file.")
	githubEndpoint      = flag.String("github-endpoint", "https://api.github.com", "GitHub's API endpoint.")
	githubTokenFile     = flag.String("github-token-file", "/etc/github/oauth", "Path to the file containing the GitHub OAuth secret.")
	webhookSecretFile   = flag.String("hmac-secret-file", "/etc/webhook/hmac", "Path to the file containing the GitHub HMAC secret.")
	sentryDsnSecretFile = flag.String("sentry-dsn-file", "/etc/sentry-dsn/sentry", "Path to the file containing the Sentry DSN url.")
	sentryTimeout       = flag.Int("sentry-timeout", 1000, "Sentry server timeout in ms. Defaults to 1 second ")
	environment         = flag.String("env", "tenant", "Environment plugin is running in. Used e.g. by Sentry for tagging.")
)

// EventHandlerCreator is a func type that creates server.GitHubEventHandler instance which is the central point for
// the plugin logic
type EventHandlerCreator func(client *github.Client) server.GitHubEventHandler

// ServerCreator is a func type that wires Server and server.GitHubEventHandler together
type ServerCreator func(hmacSecret []byte, evenHandler server.GitHubEventHandler) *server.Server

// InitPlugin instantiates logger, loads the secrets from the flags, sets context to background and starts server with
// the attached event handler.
func InitPlugin(pluginName string, newEventHandler EventHandlerCreator, newServer ServerCreator,
	helpProvider externalplugins.ExternalPluginHelpProvider) {

	// Ignore SIGTERM so that we don't drop hooks when the pod is removed.
	// We'll get SIGTERM first and then SIGKILL after our graceful termination deadline.
	signal.Ignore(syscall.SIGTERM)

	flag.Parse()

	logger := configureLogger(pluginName)

	webhookSecret, err := utils.LoadSecret(*webhookSecretFile)
	if err != nil {
		logger.WithError(err).Fatalf("unable to load webhook secret from %q", *webhookSecretFile)
	}

	oauthSecret, err := utils.LoadSecret(*githubTokenFile)
	if err != nil {
		logger.WithError(err).Fatalf("unable to load oauth token from %q", *githubTokenFile)
	}

	_, err = url.Parse(*githubEndpoint)
	if err != nil {
		logger.WithError(err).Fatalf("Must specify a valid --github-endpoint URL.")
	}

	pa := &plugins.PluginAgent{}
	if err := pa.Start(*pluginConfig); err != nil {
		logger.WithError(err).Fatalf("Error loading ike-plugins config from %q.", *pluginConfig)
	}

	ctx := context.Background()
	token := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(oauthSecret)},
	)
	githubClient := github.NewClient(oauth2.NewClient(ctx, token))

	handler := newEventHandler(githubClient)
	pluginServer := newServer(webhookSecret, handler)

	port := strconv.Itoa(*port)
	logger.Infof("Starting server on port %s", port)

	http.Handle("/", pluginServer)
	externalplugins.ServeExternalPluginHelp(http.DefaultServeMux, logger, helpProvider)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.WithError(err).Fatalf("failed to start server on port %s", port)
	}
}

func configureLogger(pluginName string) *logrus.Entry {
	logger := log.ConfigureLogrus(pluginName)

	sentryDsn, err := utils.LoadSecret(*sentryDsnSecretFile)
	if err != nil {
		logger.WithError(err).Errorf("unable to load sentry dsn from %q. No sentry integration enabled", *sentryDsnSecretFile)
	} else {
		log.AddSentryHook(logger, log.NewSentryConfiguration(string(sentryDsn), map[string]string{
			"plugin":      pluginName,
			"environment": *environment,
		}, *sentryTimeout))
	}

	return logger
}
