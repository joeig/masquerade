package goget

import (
	"bytes"
	"testing"
)

func TestResponseBody_Build(t *testing.T) {
	data := &TemplateData{
		ImportPrefix:   "import-prefix",
		VCS:            "vcs",
		RepoRoot:       "repo-root",
		ProjectWebsite: "project-website",
	}
	writer := &bytes.Buffer{}
	want := []byte(`<head>
<meta name="go-import" content="import-prefix vcs repo-root">
<meta http-equiv="refresh" content="0;URL='project-website'">
<body>
Redirecting you to the <a href="project-website">project website</a>...`)
	body := New()

	if err := body.Build(writer, data); err != nil {
		t.Error("unexpected error")
	}

	if !bytes.Equal(writer.Bytes(), want) {
		t.Error("wrong result")
	}
}
