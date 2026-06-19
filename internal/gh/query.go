package gh

const searchQuery = `query($authored: String!, $reviewing: String!, $first: Int!) {
  authored:  search(query: $authored,  type: ISSUE, first: $first) { nodes { ...pr } }
  reviewing: search(query: $reviewing, type: ISSUE, first: $first) { nodes { ...pr } }
}
fragment pr on PullRequest {
  number title url isDraft updatedAt
  repository { nameWithOwner }
  reviewDecision
  mergeable
  mergeStateStatus
  headRefName
  commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
  latestReviews(first: 50) { nodes { state } }
  latestOpinionatedReviews(first: 50) { nodes { state } }
}`

const (
	authoredFilter  = "is:open is:pr archived:false author:@me"
	reviewingFilter = "is:open is:pr archived:false review-requested:@me"
)
