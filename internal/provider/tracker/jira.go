package tracker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/shahar-caura/forge/internal/provider"
)

// Jira creates issues via the Jira Cloud REST API.
type Jira struct {
	baseURL string
	project string
	email   string
	token   string
	boardID string
	client  *http.Client
}

// New returns a Jira tracker configured with the given credentials.
// boardID is optional — if non-empty, newly created issues are moved to the active sprint.
func New(baseURL, project, email, token, boardID string) *Jira {
	return &Jira{
		baseURL: baseURL,
		project: project,
		email:   email,
		token:   token,
		boardID: boardID,
		client:  &http.Client{},
	}
}

// createIssueRequest is the JSON body for POST /rest/api/3/issue.
type createIssueRequest struct {
	Fields issueFields `json:"fields"`
}

type issueFields struct {
	Project     projectKey `json:"project"`
	Summary     string     `json:"summary"`
	IssueType   issueType  `json:"issuetype"`
	Description adfDoc     `json:"description"`
}

type projectKey struct {
	Key string `json:"key"`
}

type issueType struct {
	Name string `json:"name"`
}

// ADF (Atlassian Document Format) types for the description field.
type adfDoc struct {
	Type    string       `json:"type"`
	Version int          `json:"version"`
	Content []adfContent `json:"content"`
}

type adfContent struct {
	Type    string    `json:"type"`
	Content []adfText `json:"content"`
}

type adfText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type createIssueResponse struct {
	Key string `json:"key"`
}

// CreateIssue creates a Jira issue and returns the key and browse URL.
func (j *Jira) CreateIssue(ctx context.Context, title, body string) (*provider.Issue, error) {
	reqBody := createIssueRequest{
		Fields: issueFields{
			Project:   projectKey{Key: j.project},
			Summary:   title,
			IssueType: issueType{Name: "Task"},
			Description: adfDoc{
				Type:    "doc",
				Version: 1,
				Content: []adfContent{
					{
						Type: "paragraph",
						Content: []adfText{
							{Type: "text", Text: body},
						},
					},
				},
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("jira: marshaling request: %w", err)
	}

	url := j.baseURL + "/rest/api/3/issue"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("jira: creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(j.email + ":" + j.token))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jira: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, respBody)
	}

	var result createIssueResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("jira: parsing response: %w", err)
	}

	if result.Key == "" {
		return nil, fmt.Errorf("jira: response missing issue key")
	}

	issue := &provider.Issue{
		Key: result.Key,
		URL: j.baseURL + "/browse/" + result.Key,
	}

	if j.boardID != "" {
		sprintID, err := j.getActiveSprint(ctx)
		if err != nil {
			return nil, fmt.Errorf("jira: getting active sprint: %w", err)
		}
		if sprintID == 0 {
			slog.Warn("jira: no active sprint found, skipping move", "board_id", j.boardID)
			return issue, nil
		}
		if err := j.moveToSprint(ctx, sprintID, result.Key); err != nil {
			return nil, fmt.Errorf("jira: moving issue to sprint: %w", err)
		}
	}

	return issue, nil
}

// sprintResponse is the JSON response from the Jira Agile sprint endpoint.
type sprintResponse struct {
	Values []sprint `json:"values"`
}

type sprint struct {
	ID int `json:"id"`
}

// moveToSprintRequest is the JSON body for POST /rest/agile/1.0/sprint/{sprintID}/issue.
type moveToSprintRequest struct {
	Issues []string `json:"issues"`
}

// getActiveSprint returns the ID of the active sprint for the configured board.
// Returns 0 if no active sprint exists.
func (j *Jira) getActiveSprint(ctx context.Context) (int, error) {
	url := fmt.Sprintf("%s/rest/agile/1.0/board/%s/sprint?state=active", j.baseURL, j.boardID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(j.email + ":" + j.token))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := j.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	var result sprintResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Values) == 0 {
		return 0, nil
	}

	return result.Values[0].ID, nil
}

// moveToSprint moves an issue into the given sprint.
func (j *Jira) moveToSprint(ctx context.Context, sprintID int, issueKey string) error {
	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", j.baseURL, sprintID)
	payload, err := json.Marshal(moveToSprintRequest{Issues: []string{issueKey}})
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(j.email + ":" + j.token))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	return nil
}
