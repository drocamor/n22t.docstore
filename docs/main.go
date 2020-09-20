package main

import (
	"bufio"
	"bytes"
	"context"
	"text/template"
	"io/ioutil"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/drocamor/docstore/awsdocstore"
	"github.com/gomarkdown/markdown"
	"time"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

type docMetadata struct {
	Title, DocBody, Timestamp string
	Version int
}

const (
	tmplDocName = "doc-template.html"
	headerTmpl = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width">
    <title>%s</title>

  </head>
  <body>`
	footerTmpl = `  </body>
</html>`
)

var (
	ds *awsdocstore.AwsDocStore
)

func init() {
	ds = awsdocstore.New()
}

func firstLine(b []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Scan()
	return scanner.Text()
}

func getTemplate() (tmpl *template.Template, err error) {
	tmplDoc, err := ds.GetDoc(tmplDocName)
	if err != nil {
		return 
	}

	tmplBytes, err := ioutil.ReadAll(tmplDoc)
	if err != nil {
		return 
	}

	tmpl, err = template.New("docPage").Parse(string(tmplBytes))
	return 	
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {

	docId := request.PathParameters["docId"]

	rev, err := ds.GetDoc(docId)
	if err != nil {
		log.Printf("GetDoc error: %v", err)
		return Response{StatusCode: 404}, nil
	}

	doc, err := ioutil.ReadAll(rev)
	if err != nil {
		log.Printf("ReadAll error: %v", err)
		return Response{StatusCode: 500}, err
	}

	// If the docId includes a "." then don't render it.
	if strings.Contains(docId, ".") {
		resp := Response{
			StatusCode:      200,
			IsBase64Encoded: false,
			Body:            string(doc),
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
		}
		return resp, nil
	}

	
	// Convert the doc's markdown to HTML
	parsed := markdown.ToHTML(doc, nil, nil)

	// Get the template from the docstore
	tmpl, err := getTemplate()
	if err != nil {
		return Response{StatusCode: 500}, err
	}
	
	meta := docMetadata{
		Title: firstLine(doc),
		DocBody: string(parsed),
		Timestamp: rev.Metadata().Timestamp.Format(time.RFC850),
		Version: rev.Metadata().Id,
	}
	
	var b bytes.Buffer		

	err = tmpl.Execute(&b, meta)

	if err != nil {
		return Response{StatusCode: 500}, err
	}
	
	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            b.String(),
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
	}

	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
