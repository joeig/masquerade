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

var body = template.Must(template.New("body").Parse(bodyTemplate))

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
	return body.Execute(writer, data)
}
