package goget

import (
	"html/template"
	"io"
)

const bodyTemplate = `<head>
<meta name="go-import" content="{{.ImportPrefix}} {{.VCS}} {{.RepoRoot}}">
<meta http-equiv="refresh" content="0;URL='{{.ProjectWebsite}}'">
<body>
Redirecting you to the <a href="{{.ProjectWebsite}}">project website</a>...`

type TemplateData struct {
	ImportPrefix   string
	VCS            string
	RepoRoot       string
	ProjectWebsite string
}

type ResponseBody struct{}

func New() *ResponseBody {
	return &ResponseBody{}
}

func (r *ResponseBody) Build(writer io.Writer, data *TemplateData) error {
	body, err := template.New("body").Parse(bodyTemplate)
	if err != nil {
		return err
	}

	if err := body.Execute(writer, data); err != nil {
		return err
	}

	return nil
}
