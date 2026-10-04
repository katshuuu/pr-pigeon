package github

// Only the fields we actually use. Full schemas:
// https://docs.github.com/en/webhooks/webhook-events-and-payloads

type ghUser struct {
	Login string `json:"login"`
}

type ghRepository struct {
	FullName string `json:"full_name"`
}

type ghPullRequest struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	HTMLURL   string   `json:"html_url"`
	Merged    bool     `json:"merged"`
	User      ghUser   `json:"user"`
	Assignees []ghUser `json:"assignees"`
}

type pullRequestEvent struct {
	Action            string        `json:"action"`
	PullRequest       ghPullRequest `json:"pull_request"`
	Assignee          *ghUser       `json:"assignee"`
	RequestedReviewer *ghUser       `json:"requested_reviewer"`
	Repository        ghRepository  `json:"repository"`
	Sender            ghUser        `json:"sender"`
}

type pullRequestReviewEvent struct {
	Action string `json:"action"`
	Review struct {
		State   string `json:"state"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	} `json:"review"`
	PullRequest ghPullRequest `json:"pull_request"`
	Repository  ghRepository  `json:"repository"`
	Sender      ghUser        `json:"sender"`
}

type pullRequestReviewCommentEvent struct {
	Action  string `json:"action"`
	Comment struct {
		Body    string `json:"body"`
		Path    string `json:"path"`
		HTMLURL string `json:"html_url"`
	} `json:"comment"`
	PullRequest ghPullRequest `json:"pull_request"`
	Repository  ghRepository  `json:"repository"`
	Sender      ghUser        `json:"sender"`
}

type pullRequestReviewThreadEvent struct {
	Action string `json:"action"`
	Thread struct {
		Comments []struct {
			Path    string `json:"path"`
			HTMLURL string `json:"html_url"`
		} `json:"comments"`
	} `json:"thread"`
	PullRequest ghPullRequest `json:"pull_request"`
	Repository  ghRepository  `json:"repository"`
	Sender      ghUser        `json:"sender"`
}
