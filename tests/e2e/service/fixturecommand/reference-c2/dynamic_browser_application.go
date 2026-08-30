//go:build browsercompat

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// serveBrowserDynamic is the browser-driven H4-3B workload. The application
// knows nothing about Ardents: its normal document, form, cookies, redirect,
// streamed HTML response, and close navigation use its ordinary HTTP origin.
func serveBrowserDynamic(connection net.Conn, proofPath string) error {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	document, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = document.Body.Close()
	if document.Method != http.MethodGet || document.Host != "reference.ard" || document.URL.String() != "/" {
		return errors.New("browser-driven Publisher document request is invalid")
	}
	page := "<!doctype html><form method=post action='/publish?draft=1'><input name=title value=ardents></form><script>document.forms[0].submit()</script>"
	if _, err := fmt.Fprintf(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nSet-Cookie: before=bridge; Path=/\r\nContent-Length: %d\r\n\r\n%s", len(page), page); err != nil {
		return err
	}
	post, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(post.Body)
	_ = post.Body.Close()
	if err != nil || post.Method != http.MethodPost || post.Host != "reference.ard" || post.URL.String() != "/publish?draft=1" ||
		string(body) != "title=ardents" || post.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || !requestHasCookie(post, "before", "bridge") {
		return errors.New("browser-driven Publisher form request was changed or incomplete")
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 302 Found\r\nLocation: /timeline\r\nSet-Cookie: session=dynamic; Path=/\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return err
	}
	timeline, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = timeline.Body.Close()
	if timeline.Method != http.MethodGet || timeline.Host != "reference.ard" || timeline.URL.String() != "/timeline" ||
		!requestHasCookie(timeline, "before", "bridge") || !requestHasCookie(timeline, "session", "dynamic") {
		return errors.New("browser-driven Publisher redirect request was changed or incomplete")
	}
	firstChunk := "<script>"
	if _, err := fmt.Fprintf(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nTransfer-Encoding: chunked\r\n\r\n%x\r\n%s\r\n", len(firstChunk), firstChunk); err != nil {
		return err
	}
	time.Sleep(25 * time.Millisecond)
	secondChunk := "location.replace('/close')</script>"
	if _, err := fmt.Fprintf(connection, "%x\r\n%s\r\n0\r\n\r\n", len(secondChunk), secondChunk); err != nil {
		return err
	}
	closeRequest, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = closeRequest.Body.Close()
	if closeRequest.Method != http.MethodGet || closeRequest.Host != "reference.ard" || closeRequest.URL.String() != "/close" ||
		!requestHasCookie(closeRequest, "session", "dynamic") {
		return errors.New("browser-driven Publisher close request was changed or incomplete")
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return err
	}
	return os.WriteFile(proofPath, []byte("browser-dynamic-http\n"), 0o600)
}

func requestHasCookie(request *http.Request, name, value string) bool {
	if request == nil {
		return false
	}
	cookie, err := request.Cookie(name)
	return err == nil && cookie.Value == value && !strings.ContainsAny(cookie.Value, "\r\n")
}
