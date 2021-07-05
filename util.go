package main

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xpath"
	"golang.org/x/net/html"
)

var util = Util{}

type Util struct{}

func (u Util) xpathContains(attr string, value string) string {
	return `contains(concat(' ', ` + attr + `, ' '), ' ` + value + ` ')`
}

func (u Util) htmlInnerText(n *html.Node) string {
	return strings.TrimSpace(htmlquery.InnerText(n))
}

func (u Util) htmlInnerTextAsInt(n *html.Node) int {
	return u.atoi(u.htmlInnerText(n))
}

func (u Util) htmlInnerTextAsFloat(n *html.Node) float64 {
	return u.parseFloat(u.htmlInnerText(n))
}

func (u Util) htmlAnchorHref(n *html.Node) string {
	if n != nil {
		if a := htmlquery.QuerySelector(n, xpath.MustCompile(`//a`)); a != nil {
			return htmlquery.SelectAttr(a, "href")
		}
	}
	return ""
}

func (u Util) htmlSelectHrefLastSegment(n *html.Node) string {
	href := u.htmlAnchorHref(n)

	if href != "" {
		href = strings.TrimRight(href, "/")
		return href[strings.LastIndex(href, "/")+1:]
	}

	return ""
}

func (u Util) htmlInnerTextAndSplit(n *html.Node, sep string) []string {
	return strings.Split(u.htmlInnerText(n), sep)
}

func (u Util) htmlInnerTextFirstLine(n *html.Node) string {
	if n != nil {
		if v := u.htmlSplitLineBreak(n); 0 < len(v) {
			return strings.TrimSpace(v[0])
		}
	}
	return ""
}

func (u Util) htmlInnerTextFirstRow(n *html.Node) string {
	if v := u.htmlInnerTextAndSplit(n, " "); 0 < len(v) {
		return strings.TrimSpace(v[0])
	}
	return ""
}

func (u Util) htmlSplitLineBreak(n *html.Node) []string {
	r := regexp.MustCompile(`<br\s*/?>`)
	return r.Split(htmlquery.OutputHTML(n, false), -1)
}

func (u Util) atoi(s string) int {
	i, _ := strconv.Atoi(s)

	return i
}

func (u Util) parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)

	return f
}

func (u Util) parseFinishTime(s string) float64 {
	// s looks like "1:23.4"
	ss := strings.Split(s, ":")

	sec, _ := strconv.ParseFloat(ss[1], 64)

	if ss[0] == "0" {
		return sec
	}

	min, _ := strconv.ParseFloat(ss[0], 64)

	return (min * 60.0) + sec
}

func (u Util) openDatabase(dbFilePath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}

	return db, nil
}
