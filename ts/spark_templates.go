package ts

import (
	"bytes"
	"text/template"
)

type SVGSpark struct {
	template      *template.Template // the name for the template must be "plot"
	width, height int                // for the data on the plot, not the overall size.
}

func (s *SVGSpark) Draw(p Plot, b *bytes.Buffer) error {
	p.plt.width = s.width
	p.plt.height = s.height

	p.scaleData()

	return s.template.ExecuteTemplate(b, "plot", p.plt)
}

var SparkLine = SVGSpark{
	template: template.Must(template.New("plot").Funcs(funcMap).Parse(sparkBaseTemplate + sparkLineTemplate)),
	width:    150,
	height:   20,
}

const sparkBaseTemplate = `<?xml version="1.0"?>
<svg viewBox="0,0,155,28" class="svg" xmlns="http://www.w3.org/2000/svg" font-family="Arial, sans-serif" font-size="14px" fill="darkslategrey">
<g transform="translate(3,4)"> 
{{if .Threshold.Show}}
<rect x="0" y="{{.Threshold.Y}}" width="100" height="{{.Threshold.H}}" fill="lightgrey" fill-opacity="0.3"/>
{{end}}
{{template "data" .}}
</g>
</svg>
`
const sparkLineTemplate = `
{{define "data"}}
{{range .Data}}<polyline style="stroke: deepskyblue; fill: none; stroke-width: 1.0" points="{{range .Pts}}{{.X}},{{.Y}} {{end}}" />{{end}}
{{end}}`
