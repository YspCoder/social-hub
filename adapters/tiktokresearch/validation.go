package tiktokresearch

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var queryFields = map[string]struct{}{
	string(QueryFieldCreateDate): {}, string(QueryFieldUsername): {}, string(QueryFieldRegionCode): {},
	string(QueryFieldVideoID): {}, string(QueryFieldHashtagName): {}, string(QueryFieldKeyword): {},
	string(QueryFieldMusicID): {}, string(QueryFieldEffectID): {}, string(QueryFieldVideoLength): {},
	string(QueryFieldViewCount): {}, string(QueryFieldCommentCount): {},
}

var queryOperators = map[string]struct{}{
	string(OperatorEqual): {}, string(OperatorIn): {}, string(OperatorGreaterThan): {},
	string(OperatorGreaterThanOrEqual): {}, string(OperatorLessThan): {}, string(OperatorLessThanOrEqual): {},
}

var videoFields = map[string]struct{}{
	string(VideoFieldID): {}, string(VideoFieldDescription): {}, string(VideoFieldCreateTime): {},
	string(VideoFieldRegionCode): {}, string(VideoFieldShareCount): {}, string(VideoFieldViewCount): {},
	string(VideoFieldLikeCount): {}, string(VideoFieldCommentCount): {}, string(VideoFieldMusicID): {},
	string(VideoFieldHashtagNames): {}, string(VideoFieldUsername): {}, string(VideoFieldEffectIDs): {},
	string(VideoFieldPlaylistID): {}, string(VideoFieldVoiceToText): {}, string(VideoFieldStemVerified): {},
	string(VideoFieldFavoritesCount): {}, string(VideoFieldDuration): {}, string(VideoFieldHashtagInfoList): {},
	string(VideoFieldStickerInfoList): {}, string(VideoFieldEffectInfoList): {}, string(VideoFieldMentionList): {},
	string(VideoFieldLabel): {}, string(VideoFieldTag): {},
}

var userFields = map[string]struct{}{
	string(UserFieldDisplayName): {}, string(UserFieldBioDescription): {}, string(UserFieldAvatarURL): {},
	string(UserFieldVerified): {}, string(UserFieldFollowerCount): {}, string(UserFieldFollowingCount): {},
	string(UserFieldLikesCount): {}, string(UserFieldVideoCount): {}, string(UserFieldBioURL): {},
}

var commentFields = map[string]struct{}{
	string(CommentFieldID): {}, string(CommentFieldVideoID): {}, string(CommentFieldText): {},
	string(CommentFieldLikeCount): {}, string(CommentFieldReplyCount): {},
	string(CommentFieldParentCommentID): {}, string(CommentFieldCreateTime): {},
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validAccessToken(value string) bool {
	if !validOpaque(value, 16_384) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsSpace)
}

func validID(value string) bool {
	_, valid := parseInt64ID(value)
	return valid
}

func validResourceID(value string) bool { return value != "0" && validID(value) }

func validScopes(scopes []string) bool {
	if len(scopes) > 1 {
		return false
	}
	return len(scopes) == 0 || scopes[0] == RequiredScope
}

func containsScope(scopes []string, expected string) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func validDateRange(start, end string) bool {
	startDate, startErr := time.Parse("20060102", start)
	endDate, endErr := time.Parse("20060102", end)
	if startErr != nil || endErr != nil || startDate.Format("20060102") != start || endDate.Format("20060102") != end {
		return false
	}
	difference := endDate.Sub(startDate)
	return difference >= 0 && difference <= 30*24*time.Hour
}

func validCondition(condition Condition) bool {
	if _, valid := queryFields[string(condition.Field)]; !valid {
		return false
	}
	if _, valid := queryOperators[string(condition.Operator)]; !valid || len(condition.FieldValues) == 0 {
		return false
	}
	for _, value := range condition.FieldValues {
		if !validOpaque(value, 8192) {
			return false
		}
	}
	return true
}

func validQuery(query Query) bool {
	if len(query.And)+len(query.Or)+len(query.Not) == 0 {
		return false
	}
	for _, group := range [][]Condition{query.And, query.Or, query.Not} {
		for _, condition := range group {
			if !validCondition(condition) {
				return false
			}
		}
	}
	return true
}

func validateFieldMask[T ~string](fields []T, allowed map[string]struct{}) ([]string, bool) {
	if len(fields) == 0 || len(fields) > len(allowed) {
		return nil, false
	}
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		name := string(field)
		if _, valid := allowed[name]; !valid {
			return nil, false
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, false
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result, true
}

func validateQueryVideosRequest(input QueryVideosRequest) ([]string, error) {
	const operation = "query_videos"
	fields, valid := validateFieldMask(input.Fields, videoFields)
	if !valid || !validQuery(input.Query) || !validDateRange(input.StartDate, input.EndDate) ||
		input.MaxCount < 0 || input.MaxCount > MaximumPageSize || input.Cursor > maximumInt64Value ||
		input.Cursor > 0 && !validOpaque(input.SearchID, 4096) ||
		input.SearchID != "" && !validOpaque(input.SearchID, 4096) {
		return nil, invalidArgument(operation, "query, fields, date range, max_count, cursor, or search_id is invalid")
	}
	return fields, nil
}

func validateGetUserInfoRequest(input GetUserInfoRequest) ([]string, error) {
	fields, valid := validateFieldMask(input.Fields, userFields)
	if !valid || !validOpaque(input.Username, 256) {
		return nil, invalidArgument("get_user_info", "username or fields is invalid")
	}
	return fields, nil
}

func validateListCommentsRequest(input ListCommentsRequest) ([]string, error) {
	fields, valid := validateFieldMask(input.Fields, commentFields)
	if !valid || (input.VideoID == "") == (input.CommentID == "") ||
		input.VideoID != "" && !validResourceID(input.VideoID) || input.CommentID != "" && !validResourceID(input.CommentID) ||
		input.MaxCount < 0 || input.MaxCount > MaximumPageSize || input.Cursor > maximumInt64Value {
		return nil, invalidArgument("list_comments", "exactly one valid video_id or comment_id, valid fields, and max_count up to 100 are required")
	}
	return fields, nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Research API v2 does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Research API operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "use the operation's typed Fields member instead of socialhub.WithFields")
	}
	return nil
}

func containsVideoField(fields []VideoField, expected VideoField) bool {
	for _, field := range fields {
		if field == expected {
			return true
		}
	}
	return false
}

func containsCommentField(fields []CommentField, expected CommentField) bool {
	for _, field := range fields {
		if field == expected {
			return true
		}
	}
	return false
}

func validNonNegative(values ...*int64) bool {
	for _, value := range values {
		if value != nil && *value < 0 {
			return false
		}
	}
	return true
}

func validVideo(video Video, fields []VideoField) bool {
	if containsVideoField(fields, VideoFieldID) && !validResourceID(string(video.ID)) ||
		video.ID != "" && !validResourceID(string(video.ID)) || video.MusicID != nil && !validID(video.MusicID.String()) ||
		video.PlaylistID != nil && !validID(video.PlaylistID.String()) ||
		!validNonNegative(video.CreateTime, video.ShareCount, video.ViewCount, video.LikeCount, video.CommentCount, video.FavoritesCount, video.Duration) {
		return false
	}
	for _, identifier := range video.EffectIDs {
		if !validID(identifier.String()) {
			return false
		}
	}
	return true
}

func validUser(user User, requestedUsername string) bool {
	return (user.Username == "" || user.Username == requestedUsername) &&
		validNonNegative(user.FollowerCount, user.FollowingCount, user.LikesCount, user.VideoCount)
}

func validComment(comment Comment, fields []CommentField) bool {
	if containsCommentField(fields, CommentFieldID) && !validResourceID(comment.ID.String()) ||
		containsCommentField(fields, CommentFieldVideoID) && !validResourceID(comment.VideoID.String()) ||
		comment.ID != "" && !validResourceID(comment.ID.String()) || comment.VideoID != "" && !validResourceID(comment.VideoID.String()) ||
		comment.ParentCommentID != nil && !validID(comment.ParentCommentID.String()) ||
		!validNonNegative(comment.LikeCount, comment.ReplyCount, comment.CreateTime) {
		return false
	}
	return true
}
