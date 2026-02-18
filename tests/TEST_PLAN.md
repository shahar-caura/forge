# Forge — Test Plan

Test scenarios for forge. When a test is implemented, annotate with `<!-- tested: TestFuncName -->` to track coverage and avoid duplication.

---

## Config (`internal/config`)

- [x] Load valid `forge.yaml` with all fields populated <!-- tested: TestLoad_ValidConfig -->
- [x] Resolve `${ENV_VAR}` references from environment <!-- tested: TestLoad_EnvVarExpansion -->
- [x] Fail on missing required fields <!-- tested: TestLoad_MissingRequiredFields -->
- [ ] Fail on unresolved env var (empty or unset)
- [ ] Fail on unreachable Jira base_url
- [ ] Fail on bad `gh auth status`
- [ ] Fail on missing `claude` binary
- [x] Fail on tracker provider set but missing fields <!-- tested: TestLoad_TrackerProviderSetMissingFields -->
- [x] Fail on notifier provider set but missing webhook <!-- tested: TestLoad_NotifierProviderSetMissingWebhook -->
- [x] No validation errors when tracker/notifier unconfigured <!-- tested: TestLoad_UnconfiguredTrackerNotifierNoValidationErrors -->
- [x] CR config parsed with all fields <!-- tested: TestLoad_CRConfigParsed -->
- [x] CR config defaults applied <!-- tested: TestLoad_CRConfigDefaults -->
- [x] CR config disabled skips validation <!-- tested: TestLoad_CRConfigDisabled_NoValidation -->
- [x] CR config enabled requires comment_pattern <!-- tested: TestLoad_CRConfigEnabled_MissingPattern -->
- [x] CR config invalid fix_strategy fails <!-- tested: TestLoad_CRConfigEnabled_InvalidStrategy -->
- [x] CR config new-commit strategy accepted <!-- tested: TestLoad_CRConfigNewCommitStrategy -->

## VCS (`internal/provider/vcs`)

- [x] Create PR via `gh pr create` and return URL <!-- tested: TestCreatePR_Success, TestCreatePR_Failure -->
- [x] Commit and push <!-- tested: TestCommitAndPush_Success, TestCommitAndPush_CommitFails -->
- [x] Fetch PR review comments <!-- tested: TestGetPRComments_Success, TestGetPRComments_APIError -->
- [x] Post comment on PR <!-- tested: TestPostPRComment_Success, TestPostPRComment_Error -->
- [x] Amend and force push <!-- tested: TestAmendAndForcePush_Success, TestAmendAndForcePush_Failure -->

## Tracker (`internal/provider/tracker`)

- [x] Create issue and return key <!-- tested: TestCreateIssue_HappyPath -->
- [x] Handle auth failure (bad token/email) <!-- tested: TestCreateIssue_AuthFailure -->
- [x] Handle bad response body <!-- tested: TestCreateIssue_BadResponseBody -->
- [x] Handle context cancellation <!-- tested: TestCreateIssue_ContextCancellation -->
- [x] Handle missing issue key in response <!-- tested: TestCreateIssue_MissingKey -->

## Notifier (`internal/provider/notifier`)

- [x] Send DM notification via webhook <!-- tested: TestNotify_HappyPath -->
- [x] Handle webhook failure (bad URL, non-200) <!-- tested: TestNotify_WebhookFailure, TestNotify_BadURL -->
- [x] Handle context cancellation <!-- tested: TestNotify_ContextCancellation -->
- [x] Handle unexpected response body <!-- tested: TestNotify_UnexpectedBody -->

## Agent (`internal/provider/agent`)

- [ ] Run `claude -p` with plan as prompt in given directory
- [ ] Timeout after configured duration
- [ ] Capture stdout/stderr

## Worktree (`internal/provider/worktree`)

- [ ] Run create_cmd and capture worktree path from stdout
- [ ] Run remove_cmd for cleanup
- [ ] Fail if create_cmd exits non-zero

## Plan (`internal/plan`)

- [x] Valid frontmatter parsed <!-- tested: TestParse_ValidFrontmatter -->
- [x] No frontmatter returns body only <!-- tested: TestParse_NoFrontmatter -->
- [x] Unclosed frontmatter fails <!-- tested: TestParse_UnclosedFrontmatter -->
- [x] Empty title <!-- tested: TestParse_EmptyTitle -->
- [x] Extra fields ignored <!-- tested: TestParse_ExtraFieldsIgnored -->
- [x] Triple dash in body after frontmatter <!-- tested: TestParse_TripleDashInBodyAfterFrontmatter -->

## Pipeline (`internal/pipeline`)

- [x] Full happy path: plan → branch → agent → PR → notify <!-- tested: TestRun_HappyPath -->
- [x] Fail at step N -> skip remaining steps <!-- tested: TestRun_PlanNotFound, TestRun_AgentFails, TestRun_CommitFails, TestRun_PRCreationFails -->
- [ ] Fail at config validation -> exit 1 before any side effects
- [x] Frontmatter title used for PR and branch <!-- tested: TestRun_FrontmatterTitle_UsedForPR -->
- [x] Frontmatter title with tracker → strict branch name <!-- tested: TestRun_FrontmatterTitle_WithTracker -->
- [x] No frontmatter falls back to filename <!-- tested: TestRun_NoFrontmatter_FallbackToFilename -->
- [x] Tracker nil — skips issue creation <!-- tested: TestRun_TrackerNil_SkipsIssueCreation -->
- [x] Tracker creates issue — key stored in state <!-- tested: TestRun_TrackerCreatesIssue -->
- [x] Tracker fails — pipeline fails <!-- tested: TestRun_TrackerFails_PipelineFails -->
- [x] Notifier nil — skips notification <!-- tested: TestRun_NotifierNil_SkipsNotification -->
- [x] Notifier called on success with PR URL <!-- tested: TestRun_NotifierCalled_OnSuccess -->
- [x] Notifier called on success with issue URL <!-- tested: TestRun_NotifierCalled_OnSuccess_WithIssue -->
- [x] Notifier called on failure (best-effort) <!-- tested: TestRun_NotifierCalled_OnFailure -->
- [x] Notifier failure fails pipeline <!-- tested: TestRun_NotifierFailure_FailsPipeline -->
- [x] State tracking happy path <!-- tested: TestRun_StateTrackingHappyPath -->
- [x] Resume skips completed steps <!-- tested: TestRun_ResumeSkipsCompletedSteps -->
- [x] Resume after agent failure <!-- tested: TestRun_ResumeAfterAgentFailure -->
- [x] Worktree preserved on failure <!-- tested: TestRun_WorktreePreservedOnFailure -->
- [x] Worktree cleaned on success <!-- tested: TestRun_WorktreeCleanedOnSuccess -->
- [x] Resume with missing worktree <!-- tested: TestRun_ResumeWithMissingWorktree -->
- [x] Artifacts stored in state <!-- tested: TestRun_ArtifactsStoredInState -->
- [x] CR loop happy path — poll, fix, amend push <!-- tested: TestRun_CRLoop_HappyPath -->
- [x] CR loop disabled — steps skipped <!-- tested: TestRun_CRLoop_Disabled -->
- [x] CR loop poll timeout — pipeline fails <!-- tested: TestRun_CRLoop_PollTimeout -->
- [x] CR loop new-commit strategy <!-- tested: TestRun_CRLoop_NewCommitStrategy -->
- [x] CR loop resume from fix step <!-- tested: TestRun_CRLoop_Resume -->

## Branch Naming (`internal/pipeline`)

- [x] SlugFromTitle converts titles to kebab-case <!-- tested: TestSlugFromTitle -->
- [x] BranchName with issue key produces strict format <!-- tested: TestBranchName -->
- [x] BranchName without issue key uses forge/ prefix <!-- tested: TestBranchName_NoIssueKey -->
- [x] ValidateBranchName validates strict pattern <!-- tested: TestValidateBranchName -->

## E2E Flows

- [ ] `forge run tests/plans/hello-world.md` — creates Jira issue, branch, PR, sends Slack notification
- [ ] `forge run` with missing config -> prints validation errors, exits 1
- [ ] `forge run` with unreachable Jira -> fails fast, notifies Slack
