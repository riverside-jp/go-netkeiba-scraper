package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xpath"
	"golang.org/x/net/html"
	"golang.org/x/xerrors"
)

type raceInformationRecord struct {
	id              int
	name            string
	racetrack       string
	raceNumber      int
	surface         string
	course          string
	distance        int
	weather         string
	surfaceState    string
	raceStart       string
	surfaceScore    int
	date            string
	placeDetail     string
	class           int
	classDetail     string
}

type payoffRecord struct {
	raceID       int
	ticketType   int
	horseNumber  string
	payoff       float64
	popularity   int
}

type raceResultRecord struct {
	raceID        int
	orderOfFinish string
	frameNumber   int
	horseNumber   int
	horseID       string
	horseName     string
	sex           string
	age           int
	basisWeight   float64
	jockeyID      string
	jockeyName    string
	finishingTime string
	length        string
	speedFigure   int
	pass          string
	lastPhase     float64
	odds          float64
	popularity    int
	horseWeight   string
	remark        string
	stable        string
	trainerID     string
	ownerID       string
	earningMoney  float64
}

func buildRaceInformationRecord(id int, doc *html.Node) (*raceInformationRecord, error) {
	record := &raceInformationRecord{id: id}

	raceData := htmlquery.QuerySelector(doc, xpath.MustCompile(`//dl[` + util.xpathContains("@class", "racedata") + `]`))
	if raceData == nil {
		return nil, xerrors.New(`Missing race data from HTML`)
	}

	if dt, err := htmlquery.Query(raceData, "//dt"); err == nil{
		if i, err := strconv.Atoi(strings.TrimRight(util.htmlInnerText(dt), " R")); err == nil {
			record.raceNumber = i
		}
	} else {
		return nil, xerrors.Errorf("Error XPATH query: %+w", err)
	}

	if h1, err := htmlquery.Query(raceData, "//h1"); err == nil {
		record.name = util.htmlInnerText(h1)
	} else {
		return nil, xerrors.Errorf("Error XPATH query: %+w", err)
	}

	if span, err := htmlquery.Query(raceData, "//span"); err == nil {
		s := util.htmlInnerTextAndSplit(span, "/")

		r1 := regexp.MustCompile(`([^\d]+)([\d]+)m`)
		m1 := r1.FindAllStringSubmatch(s[0], -1)

		record.surface = string([]rune(m1[0][1])[:1])
		record.course = string([]rune(m1[0][1])[1:])

		if i, err := strconv.Atoi(m1[0][2]); err == nil {
			record.distance = i
		}

		r2 := regexp.MustCompile(`.* \: (.+)`)

		record.weather = strings.Replace(strings.TrimSpace(s[1]), "天候 : ","", -1)
		record.surfaceState = r2.ReplaceAllString(strings.TrimSpace(s[2]), "$1")
		record.raceStart = strings.Replace(strings.TrimSpace(s[3]), "発走 : ","", -1)
	} else {
		return nil, xerrors.Errorf("Error XPATH query: %+w", err)
	}

	if p := htmlquery.QuerySelector(doc, xpath.MustCompile(`//p[` + util.xpathContains("@class", "smalltxt") + `]`)); p != nil {
		s := util.htmlInnerTextAndSplit(p, " ")
		t, _ := time.Parse("2006年1月2日", s[0])

		r := regexp.MustCompile(`.*(中京|中山|京都|函館|小倉|新潟|札幌|東京|福島|阪神).*`)

		record.racetrack = r.ReplaceAllString(s[1], "$1")
		record.date = t.Format("2006-01-02")
		record.placeDetail = s[1]
		record.classDetail = s[2]
		record.class = determineRaceClass(strings.Split(s[2], " ")[0])
	} else {
		return nil, xerrors.New(`Missing p[@class="smalltxt"]`)
	}

	if td := htmlquery.QuerySelector(doc, xpath.MustCompile(`//table[@summary="馬場情報"]/tbody/tr/th[text()='馬場指数']/following-sibling::td`)); td != nil {
		t := util.htmlInnerText(td)

		if i, err := strconv.Atoi(t[:strings.Index(t, " ")]); err == nil {
			record.surfaceScore = i
		}
	}

	return record, nil
}

func buildPayoffRecords(id int, doc *html.Node) ([]*payoffRecord, error) {
	tr := htmlquery.QuerySelectorAll(doc, xpath.MustCompile(`//table[` + util.xpathContains("@class", "pay_table_01") + `]//tr`))
	if len(tr) == 0 {
		return nil, xerrors.New(`Missing payoff table from HTML`)
	}

	var records []*payoffRecord

	for i := 0; i < len(tr); i++ {
		th := htmlquery.QuerySelector(tr[i], xpath.MustCompile(`//th`))
		td := htmlquery.QuerySelectorAll(tr[i], xpath.MustCompile(`//td`))

		horse := util.htmlSplitLineBreak(td[0])
		payoff := util.htmlSplitLineBreak(td[1])
		popularity := util.htmlSplitLineBreak(td[2])

		for i := 0; i < len(horse); i++ {
			record := &payoffRecord{
				raceID: id,
				ticketType: determineTicketType(util.htmlInnerText(th)),
				horseNumber: horse[i],
				payoff: util.parseFloat(payoff[i]),
				popularity: util.atoi(popularity[i]),
			}

			records = append(records, record)
		}
	}

	return records, nil
}

func buildRaceResultRecords(id int, doc *html.Node) ([]*raceResultRecord, error) {
	tr := htmlquery.QuerySelectorAll(doc, xpath.MustCompile(`//table[` + util.xpathContains("@class", "race_table_01") + `]//tr`))

	// first line is table header
	if len(tr) < 2 {
		return nil, xerrors.New(`Race result not found, or is invalid`)
	}

	var records []*raceResultRecord

	for i := 1; i < len(tr); i++ {
		td := htmlquery.QuerySelectorAll(tr[i], xpath.MustCompile(`//td`))

		sex := string([]rune(util.htmlInnerText(td[4]))[:1])
		age, _ := strconv.Atoi(string([]rune(util.htmlInnerText(td[4]))[1:]))

		record := &raceResultRecord{
			raceID: id,
			orderOfFinish: util.htmlInnerText(td[0]),
			frameNumber: util.htmlInnerTextAsInt(td[1]),
			horseNumber: util.htmlInnerTextAsInt(td[2]),
			horseID: util.htmlSelectHrefLastSegment(td[3]),
			horseName: util.htmlInnerText(td[3]),
			sex: sex,
			age: age,
			basisWeight: util.htmlInnerTextAsFloat(td[5]),
			jockeyID: util.htmlSelectHrefLastSegment(td[6]),
			jockeyName: util.htmlInnerText(td[6]),
			finishingTime: util.htmlInnerText(td[7]),
			length: util.htmlInnerText(td[8]),
			speedFigure: util.htmlInnerTextAsInt(td[9]),
			pass: util.htmlInnerText(td[10]),
			lastPhase: util.htmlInnerTextAsFloat(td[11]),
			odds: util.htmlInnerTextAsFloat(td[12]),
			popularity: util.htmlInnerTextAsInt(td[13]),
			horseWeight: util.htmlInnerText(td[14]),
			remark: util.htmlInnerText(td[17]),
			stable: string([]rune(util.htmlInnerText(td[18]))[1:2]),
			trainerID: util.htmlSelectHrefLastSegment(td[18]),
			ownerID: util.htmlSelectHrefLastSegment(td[19]),
			earningMoney: util.htmlInnerTextAsFloat(td[20]),
		}

		records = append(records, record)
	}

	return records, nil
}

func determineTicketType(s string) int {
	switch s {
	case "複勝":
		return 1
	case "枠連":
		return 2
	case "馬連":
		return 3
	case "ワイド":
		return 4
	case "馬単":
		return 5
	case "三連複":
		return 6
	case "三連単":
		return 7
	case "単勝":
		fallthrough
	default:
		return -1
	}
}

func determineRaceClass(s string) int {
	switch s {
	case "2歳新馬": fallthrough
	case "3歳新馬": fallthrough
	case "2歳未勝利": fallthrough
	case "3歳未勝利":
		return 0
	case "2歳1勝クラス": fallthrough
	case "3歳1勝クラス": fallthrough
	case "2歳500万下": fallthrough
	case "3歳500万下":
		return 1
	case "3歳以上1勝クラス": fallthrough
	case "3歳以上500万下": fallthrough
	case "4歳以上1勝クラス": fallthrough
	case "4歳以上500万下": fallthrough
	case "3歳以上2勝クラス": fallthrough
	case "4歳以上2勝クラス": fallthrough
	case "3歳以上1000万下": fallthrough
	case "4歳以上1000万下":
		return 2
	case "2歳オープン": fallthrough
	case "3歳オープン": fallthrough
	case "3歳以上オープン": fallthrough
	case "4歳以上オープン": fallthrough
	case "3歳以上1600万下": fallthrough
	case "4歳以上1600万下": fallthrough
	case "4歳以上3勝クラス":
		return 3
	case "障害3歳以上オープン": fallthrough
	case "障害3歳以上未勝利": fallthrough
	case "障害4歳以上オープン": fallthrough
	case "障害4歳以上未勝利":
		return 4
	default:
		return -1
	}
}