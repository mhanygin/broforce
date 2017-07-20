package bus

const (
	TimerEvent            = "TIMER"
	GithubHookEvent       = "GITHUB"
	GitlabHookEvent       = "GITLAB"
	ServeCmdEvent         = "SERVE"
	ServeCmdWithDataEvent = "SERVE_WITH_DATA"
	OutdatedEvent         = "OUTDATED"
	SlackMsgEvent         = "SLACK_MESSAGE"
	SlackPostEvent        = "SLACK_POST_MESSAGE"
	TelegramMsgEvent      = "TELEGRAM_MESSAGE"
	JiraHookEvent         = "JIRA"
	UnknownEvent          = "UNKNOWN"
)

const (
	JsonCoding = "json"
)
