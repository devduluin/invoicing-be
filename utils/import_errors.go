package utils

import "strings"

// ImportErrors is every problem an import file has, one message per problem ("Row 3: …"), so the user
// fixes the whole file in one go instead of uploading it again for each error. Nothing of the file is
// saved when it is returned. Controllers answer it as a 422 with all messages in `errors`.
type ImportErrors struct{ Messages []string }

func (e *ImportErrors) Error() string { return strings.Join(e.Messages, "; ") }
