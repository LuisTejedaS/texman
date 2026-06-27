package ui

// EditMode describes which inline editor is currently active in the request panel.
type EditMode int

const (
	EditModeNone              EditMode = iota
	EditModeBody                       // editing request body with a textarea
	EditModeHeaderList                 // browsing the header key-value list
	EditModeHeaderValue                // editing an existing header's value
	EditModeNewHeaderKey               // typing a brand-new header's key
	EditModeNewHeaderValue             // typing a brand-new header's value
	EditModeNewReqName                 // new-request wizard – step 1: name
	EditModeNewReqMethod               // new-request wizard – step 2: HTTP method
	EditModeNewReqURL                  // new-request wizard – step 3: URL
	EditModeDeleteConfirm              // waiting for y/n before deleting a request
	EditModeMethod                     // editing the HTTP method of an existing request
	EditModeURL                        // editing the URL of an existing request
	EditModeNewCollName                // new-collection wizard – step 1: name
	EditModeRenameCollName             // rename an existing collection
	EditModeDeleteCollConfirm          // waiting for y/n before deleting a collection
	EditModeImportCollPath             // import collection JSON from a file path
	EditModeExportCollPath             // export selected collection JSON to a file path
)
