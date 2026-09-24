package repository

// listBlobExclude — columns list endpoints must never SELECT unless explicitly requested via
// ?fields=. Both are base64 data: URIs (attachment_data can be a whole file, signature_data an
// image) that no list UI ever renders — they're only used on FindByID (the detail page). Without
// this, GORM's PreserveAssociations path fetches every column on the model for every row of every
// page, multiplying response size and DB I/O for no reason a list view needs.
var listBlobExclude = []string{"attachment_data", "signature_data"}
