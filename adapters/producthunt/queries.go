package producthunt

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

const userFields = `
id name username headline createdAt followersCount followingCount
isFollowing isMaker isViewer profileImage(size: 256) coverImage(width: 1200, height: 400)
twitterUsername url websiteUrl`

const mediaFields = `type url videoUrl`

const postFields = `
id name slug tagline description createdAt featuredAt scheduledAt
dailyRank weeklyRank monthlyRank yearlyRank commentsCount votesCount reviewsCount reviewsRating
isCollected isVoted url website userId
makers {` + userFields + `}
media {` + mediaFields + `}
productLinks { type url }
thumbnail {` + mediaFields + `}
user {` + userFields + `}`

const topicFields = `
id name slug description createdAt followersCount postsCount isFollowing image(width: 512, height: 512) url`

const collectionFields = `
id name tagline description coverImage(width: 1200, height: 630) createdAt featuredAt
followersCount isFollowing url userId user {` + userFields + `}`

const commentFields = `
id body createdAt url parentId isVoted votesCount userId
parent { id body createdAt url parentId isVoted votesCount userId }
user {` + userFields + `}`

const listPostsQuery = `
query ListPosts(
  $first: Int, $after: String, $last: Int, $before: String,
  $featured: Boolean, $order: PostsOrder, $postedAfter: DateTime, $postedBefore: DateTime,
  $topic: String, $twitterUrl: String, $url: String
) {
  posts(
    first: $first, after: $after, last: $last, before: $before,
    featured: $featured, order: $order, postedAfter: $postedAfter, postedBefore: $postedBefore,
    topic: $topic, twitterUrl: $twitterUrl, url: $url
  ) {
    edges { cursor node {` + postFields + `} }
    pageInfo { endCursor hasNextPage hasPreviousPage startCursor }
    totalCount
  }
}`

const getPostQuery = `
query GetPost($id: ID, $slug: String) {
  post(id: $id, slug: $slug) {` + postFields + `}
}`

const listTopicsQuery = `
query ListTopics(
  $first: Int, $after: String, $last: Int, $before: String,
  $followedByUserID: ID, $order: TopicsOrder, $query: String
) {
  topics(
    first: $first, after: $after, last: $last, before: $before,
    followedByUserid: $followedByUserID, order: $order, query: $query
  ) {
    edges { cursor node {` + topicFields + `} }
    pageInfo { endCursor hasNextPage hasPreviousPage startCursor }
    totalCount
  }
}`

const getTopicQuery = `
query GetTopic($id: ID, $slug: String) {
  topic(id: $id, slug: $slug) {` + topicFields + `}
}`

const listCollectionsQuery = `
query ListCollections(
  $first: Int, $after: String, $last: Int, $before: String,
  $featured: Boolean, $order: CollectionsOrder, $postID: ID, $userID: ID
) {
  collections(
    first: $first, after: $after, last: $last, before: $before,
    featured: $featured, order: $order, postId: $postID, userId: $userID
  ) {
    edges { cursor node {` + collectionFields + `} }
    pageInfo { endCursor hasNextPage hasPreviousPage startCursor }
    totalCount
  }
}`

const getCollectionQuery = `
query GetCollection($id: ID, $slug: String) {
  collection(id: $id, slug: $slug) {` + collectionFields + `}
}`

const listPostCommentsQuery = `
query ListPostComments(
  $id: ID, $slug: String, $first: Int, $after: String, $last: Int, $before: String,
  $order: CommentsOrder
) {
  post(id: $id, slug: $slug) {
    comments(first: $first, after: $after, last: $last, before: $before, order: $order) {
      edges { cursor node {` + commentFields + `} }
      pageInfo { endCursor hasNextPage hasPreviousPage startCursor }
      totalCount
    }
  }
}`

const getCommentQuery = `
query GetComment($id: ID!) {
  comment(id: $id) {` + commentFields + `}
}`

const getUserQuery = `
query GetUser($id: ID, $username: String) {
  user(id: $id, username: $username) {` + userFields + `}
}`

const getViewerQuery = `
query GetViewer {
  viewer { user {` + userFields + `} }
}`

func (client *Client) ListPosts(ctx context.Context, input ListPostsRequest, options ...socialhub.CallOption) (PostsResponse, error) {
	if !validPagination(input.Page) || !validPostsOrder(input.Order) ||
		!validOptionalOpaque(input.Topic, 256) || (input.Topic != "" && !validSlug(input.Topic)) ||
		!validOptionalOpaque(input.TwitterURL, 4096) || (input.TwitterURL != "" && !validWebURL(input.TwitterURL)) ||
		!validOptionalOpaque(input.URL, 4096) || (input.URL != "" && !validWebURL(input.URL)) ||
		(!input.PostedAfter.IsZero() && !input.PostedBefore.IsZero() && input.PostedAfter.After(input.PostedBefore)) {
		return PostsResponse{}, invalidArgument("list_posts", "pagination, order, filters, or date range is invalid")
	}
	variables := paginationVariables(input.Page)
	setOptional(variables, "featured", input.Featured)
	setString(variables, "order", string(input.Order))
	setTime(variables, "postedAfter", input.PostedAfter)
	setTime(variables, "postedBefore", input.PostedBefore)
	setString(variables, "topic", input.Topic)
	setString(variables, "twitterUrl", input.TwitterURL)
	setString(variables, "url", input.URL)
	var data struct {
		Posts *Connection[Post] `json:"posts"`
	}
	meta, raw, err := client.doGraphQL(ctx, "list_posts", listPostsQuery, variables, &data, options...)
	response := PostsResponse{Meta: meta, Raw: raw}
	if data.Posts != nil {
		response.Posts = *data.Posts
	}
	if err != nil {
		return response, err
	}
	if data.Posts == nil {
		return response, platformContractError("list_posts", "Product Hunt omitted the non-null posts connection")
	}
	if !validConnection(data.Posts, func(value Post) string { return value.ID }) {
		return response, platformContractError("list_posts", "Product Hunt returned an invalid posts connection")
	}
	return response, nil
}

func (client *Client) GetPost(ctx context.Context, input ObjectLookup, options ...socialhub.CallOption) (PostResponse, error) {
	if !validObjectLookup(input) {
		return PostResponse{}, invalidArgument("get_post", "exactly one valid post ID or slug is required")
	}
	var data struct {
		Post *Post `json:"post"`
	}
	meta, raw, err := client.doGraphQL(ctx, "get_post", getPostQuery, lookupVariables(input), &data, options...)
	response := PostResponse{Post: data.Post, Meta: meta, Raw: raw}
	if err != nil {
		return response, err
	}
	if data.Post == nil {
		return response, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(data.Post.ID, 256) {
		return response, platformContractError("get_post", "Product Hunt returned an invalid post")
	}
	return response, nil
}

func (client *Client) ListTopics(ctx context.Context, input ListTopicsRequest, options ...socialhub.CallOption) (TopicsResponse, error) {
	if !validPagination(input.Page) || !validTopicsOrder(input.Order) ||
		!validOptionalOpaque(input.FollowedByUserID, 256) || !validOptionalOpaque(input.Query, 512) {
		return TopicsResponse{}, invalidArgument("list_topics", "pagination, order, user ID, or query is invalid")
	}
	variables := paginationVariables(input.Page)
	setString(variables, "followedByUserID", input.FollowedByUserID)
	setString(variables, "order", string(input.Order))
	setString(variables, "query", input.Query)
	var data struct {
		Topics *Connection[Topic] `json:"topics"`
	}
	meta, raw, err := client.doGraphQL(ctx, "list_topics", listTopicsQuery, variables, &data, options...)
	response := TopicsResponse{Meta: meta, Raw: raw}
	if data.Topics != nil {
		response.Topics = *data.Topics
	}
	if err != nil {
		return response, err
	}
	if data.Topics == nil {
		return response, platformContractError("list_topics", "Product Hunt omitted the non-null topics connection")
	}
	if !validConnection(data.Topics, func(value Topic) string { return value.ID }) {
		return response, platformContractError("list_topics", "Product Hunt returned an invalid topics connection")
	}
	return response, nil
}

func (client *Client) GetTopic(ctx context.Context, input ObjectLookup, options ...socialhub.CallOption) (TopicResponse, error) {
	if !validObjectLookup(input) {
		return TopicResponse{}, invalidArgument("get_topic", "exactly one valid topic ID or slug is required")
	}
	var data struct {
		Topic *Topic `json:"topic"`
	}
	meta, raw, err := client.doGraphQL(ctx, "get_topic", getTopicQuery, lookupVariables(input), &data, options...)
	response := TopicResponse{Topic: data.Topic, Meta: meta, Raw: raw}
	if err != nil {
		return response, err
	}
	if data.Topic == nil {
		return response, platformError("get_topic", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(data.Topic.ID, 256) {
		return response, platformContractError("get_topic", "Product Hunt returned an invalid topic")
	}
	return response, nil
}

func (client *Client) ListCollections(ctx context.Context, input ListCollectionsRequest, options ...socialhub.CallOption) (CollectionsResponse, error) {
	if !validPagination(input.Page) || !validCollectionsOrder(input.Order) ||
		!validOptionalOpaque(input.PostID, 256) || !validOptionalOpaque(input.UserID, 256) {
		return CollectionsResponse{}, invalidArgument("list_collections", "pagination, order, post ID, or user ID is invalid")
	}
	variables := paginationVariables(input.Page)
	setOptional(variables, "featured", input.Featured)
	setString(variables, "order", string(input.Order))
	setString(variables, "postID", input.PostID)
	setString(variables, "userID", input.UserID)
	var data struct {
		Collections *Connection[Collection] `json:"collections"`
	}
	meta, raw, err := client.doGraphQL(ctx, "list_collections", listCollectionsQuery, variables, &data, options...)
	response := CollectionsResponse{Meta: meta, Raw: raw}
	if data.Collections != nil {
		response.Collections = *data.Collections
	}
	if err != nil {
		return response, err
	}
	if data.Collections == nil {
		return response, platformContractError("list_collections", "Product Hunt omitted the non-null collections connection")
	}
	if !validConnection(data.Collections, func(value Collection) string { return value.ID }) {
		return response, platformContractError("list_collections", "Product Hunt returned an invalid collections connection")
	}
	return response, nil
}

func (client *Client) GetCollection(ctx context.Context, input ObjectLookup, options ...socialhub.CallOption) (CollectionResponse, error) {
	if !validObjectLookup(input) {
		return CollectionResponse{}, invalidArgument("get_collection", "exactly one valid collection ID or slug is required")
	}
	var data struct {
		Collection *Collection `json:"collection"`
	}
	meta, raw, err := client.doGraphQL(ctx, "get_collection", getCollectionQuery, lookupVariables(input), &data, options...)
	response := CollectionResponse{Collection: data.Collection, Meta: meta, Raw: raw}
	if err != nil {
		return response, err
	}
	if data.Collection == nil {
		return response, platformError("get_collection", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(data.Collection.ID, 256) {
		return response, platformContractError("get_collection", "Product Hunt returned an invalid collection")
	}
	return response, nil
}

func (client *Client) ListPostComments(ctx context.Context, input ListPostCommentsRequest, options ...socialhub.CallOption) (CommentsResponse, error) {
	if !validObjectLookup(input.Post) || !validPagination(input.Page) || !validCommentsOrder(input.Order) {
		return CommentsResponse{}, invalidArgument("list_post_comments", "post lookup, pagination, or order is invalid")
	}
	variables := lookupVariables(input.Post)
	addPaginationVariables(variables, input.Page)
	setString(variables, "order", string(input.Order))
	var data struct {
		Post *struct {
			Comments *Connection[Comment] `json:"comments"`
		} `json:"post"`
	}
	meta, raw, err := client.doGraphQL(ctx, "list_post_comments", listPostCommentsQuery, variables, &data, options...)
	response := CommentsResponse{Meta: meta, Raw: raw}
	if data.Post != nil && data.Post.Comments != nil {
		response.Comments = *data.Post.Comments
	}
	if err != nil {
		return response, err
	}
	if data.Post == nil {
		return response, platformError("list_post_comments", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if data.Post.Comments == nil {
		return response, platformContractError("list_post_comments", "Product Hunt omitted the non-null comments connection")
	}
	if !validConnection(data.Post.Comments, func(value Comment) string { return value.ID }) {
		return response, platformContractError("list_post_comments", "Product Hunt returned an invalid comments connection")
	}
	return response, nil
}

func (client *Client) GetComment(ctx context.Context, commentID string, options ...socialhub.CallOption) (CommentResponse, error) {
	if !validOpaque(commentID, 256) {
		return CommentResponse{}, invalidArgument("get_comment", "a valid comment ID is required")
	}
	var data struct {
		Comment *Comment `json:"comment"`
	}
	meta, raw, err := client.doGraphQL(ctx, "get_comment", getCommentQuery, map[string]any{"id": commentID}, &data, options...)
	response := CommentResponse{Comment: data.Comment, Meta: meta, Raw: raw}
	if err != nil {
		return response, err
	}
	if data.Comment == nil {
		return response, platformError("get_comment", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(data.Comment.ID, 256) {
		return response, platformContractError("get_comment", "Product Hunt returned an invalid comment")
	}
	return response, nil
}

func (client *Client) GetUser(ctx context.Context, input UserLookup, options ...socialhub.CallOption) (UserResponse, error) {
	if !validUserLookup(input) {
		return UserResponse{}, invalidArgument("get_user", "exactly one valid user ID or username is required")
	}
	variables := make(map[string]any, 1)
	setString(variables, "id", input.ID)
	setString(variables, "username", input.Username)
	var data struct {
		User *User `json:"user"`
	}
	meta, raw, err := client.doGraphQL(ctx, "get_user", getUserQuery, variables, &data, options...)
	response := UserResponse{User: data.User, Meta: meta, Raw: raw}
	if err != nil {
		return response, err
	}
	if data.User == nil {
		return response, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(data.User.ID, 256) {
		return response, platformContractError("get_user", "Product Hunt returned an invalid user")
	}
	return response, nil
}

func (client *Client) GetViewer(ctx context.Context, options ...socialhub.CallOption) (UserResponse, error) {
	var data struct {
		Viewer *struct {
			User *User `json:"user"`
		} `json:"viewer"`
	}
	meta, raw, err := client.doGraphQL(ctx, "get_viewer", getViewerQuery, map[string]any{}, &data, options...)
	response := UserResponse{Meta: meta, Raw: raw}
	if data.Viewer != nil {
		response.User = data.Viewer.User
	}
	if err != nil {
		return response, err
	}
	if data.Viewer == nil || data.Viewer.User == nil {
		return response, &socialhub.Error{
			Code: socialhub.CodePermissionDenied, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: "get_viewer",
			PlatformMessage: "Viewer requires an access token with user context", ApprovalURL: dashboardURL,
		}
	}
	if !validOpaque(data.Viewer.User.ID, 256) {
		return response, platformContractError("get_viewer", "Product Hunt returned an invalid viewer user")
	}
	return response, nil
}

func paginationVariables(value Pagination) map[string]any {
	variables := make(map[string]any, 4)
	addPaginationVariables(variables, value)
	return variables
}

func addPaginationVariables(variables map[string]any, value Pagination) {
	if value.First > 0 {
		variables["first"] = value.First
	}
	setString(variables, "after", value.After)
	if value.Last > 0 {
		variables["last"] = value.Last
	}
	setString(variables, "before", value.Before)
}

func lookupVariables(value ObjectLookup) map[string]any {
	variables := make(map[string]any, 1)
	setString(variables, "id", value.ID)
	setString(variables, "slug", value.Slug)
	return variables
}

func setString(variables map[string]any, name, value string) {
	if value != "" {
		variables[name] = value
	}
}

func setOptional[T any](variables map[string]any, name string, value *T) {
	if value != nil {
		variables[name] = *value
	}
}

func setTime(variables map[string]any, name string, value time.Time) {
	if !value.IsZero() {
		variables[name] = value.UTC().Format(time.RFC3339)
	}
}
