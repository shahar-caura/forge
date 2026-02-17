# Example workflow:
# /do [plan.md]
# create jira issue
# create git worktree (new branch with name enforcement based on jira issue)
# implement plan.md
# commit, push, create pr
# read CR comment from pr (takes ~2-3 minutes to be generated)
# fix according to CR, commit amend + force with lease
# post response comment (to CR comment) on gh
# send slack notification (dm) - pr is ready for human review (with link)
# human approves -> sends pr in slack pr channel
# if human needs changes - either write in new gh comment manually and tag claude, or think of different solution

# I want this workflow to be:
# generic (optional steps, support gh or gitlabs, jira or monday etc).
# reliable - not mcp. either bash or go
# secure - each step should have all permissions it needs, and only them.
# configurable - security level could include everything from "can do anything" to "tool can only do npm install shadcn:*"
# support parallelism - use worktrees to develop multiple features.
# smart queues - if there are 3 plans to implement, and plan#3 depends on plan#1 and plan#2, only run it after they both merged to ROOT branch (dev by default)
# remote interaction - keep working even if computer is locked (claude code...); trigger from mobile phone, run remotely
# fire-and-forget - non interactive. once started running, should only send alerts to user if errors or if success (done, need last review)

# consider creating a claude command that receives a prd.md and runs a go script
# Please research most common available solutions