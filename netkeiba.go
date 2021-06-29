package main

import (
	"database/sql"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xpath"
	"github.com/gocolly/colly/v2"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"golang.org/x/xerrors"
)

func findRaceSchedulePages(url string) (pages []string, prevMonthPage string, err error) {
	c := colly.NewCollector()

	c.OnHTML("div.race_calendar table a", func(e *colly.HTMLElement) {
		pages = append(pages, e.Attr("href"))
	})

	c.OnHTML("div.race_calendar .rev a:last-child", func(e *colly.HTMLElement) {
		prevMonthPage = e.Attr("href")
	})

	c.OnRequest(func(request *colly.Request) {
		log.Println("Sending request to " + url)
	})

	if err := c.Visit(url + "/?pid=race_top"); err != nil {
		return nil, "", err
	}

	return pages, prevMonthPage, nil
}

func findRacePagesOfOneDay(baseURL string, path string) ([]string, error) {
	var races []string

	c := colly.NewCollector()

	c.OnHTML("dl.race_top_data_info dd > a", func(e *colly.HTMLElement) {
		races = append(races, baseURL+e.Attr("href"))
	})

	c.OnRequest(func(request *colly.Request) {
		log.Println("Sending request to " + baseURL + path)
	})

	if err := c.Visit(baseURL + path); err != nil {
		return nil, err
	}

	return races, nil
}

func loginToNetkeibaCom(loginURL string, id string, password string) error {
	log.Println("Trying to login to netkeiba.com, login_id is " + id)

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return err
	}

	http.DefaultClient.Jar = jar

	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	values := url.Values{}
	values.Set("login_id", id)
	values.Set("pswd", password)
	values.Set("pid", "login")
	values.Set("action", "auth")

	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.Println("Sending request to " + loginURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusFound {
		return xerrors.Errorf("login failure, status code is %d", resp.StatusCode)
	}

	log.Println("Succeeded to login")

	return nil
}

func dumpWebPageAsHTMLFile(dumpDir string, url string) error {
	log.Println("Sending request to " + url)

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return xerrors.Errorf("request failure, status code is %d", err)
	}

	// EUC-JP -> UTF-8
	r := transform.NewReader(resp.Body, japanese.EUCJP.NewDecoder())

	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}

	filename := filepath.Join(dumpDir, determineDumpHTMLFilenameFromURL(url))

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0666))
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(string(bytes)); err != nil {
		return err
	}

	log.Printf("Dumped %s to %s", url, filename)

	return nil
}

func determineDumpHTMLFilenameFromURL(url string) string {
	// url looks like "https://db.netkeiba.com/race/202105020305/"
	s := strings.Split(strings.TrimRight(url, "/"), "/")

	return s[len(s)-1] + ".html"
}

func importRaceData(db *sql.DB, filePath string) error {
	id, _ := strconv.Atoi(strings.TrimSuffix(filepath.Base(filePath), ".html"))

	doc, err := htmlquery.LoadDoc(filePath)
	if err != nil {
		return err
	}

	race, err := buildRaceRecord(id, doc)
	if err != nil {
		return xerrors.Errorf("build race information record failure: %+w", err)
	}

	payouts, err := buildPayoutRecords(id, doc)
	if err != nil {
		return xerrors.Errorf("build payout records failure: %+w", err)
	}

	results, err := buildResultRecords(id, doc)
	if err != nil {
		return xerrors.Errorf("build result records failure: %+w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	s1, err := tx.Prepare(`INSERT OR REPLACE INTO race VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return err
	}
	defer s1.Close()

	if _, err := s1.Exec(
		race.id,
		race.name,
		race.course,
		race.number,
		race.surface,
		race.direction,
		race.distance,
		race.weather,
		race.surfaceState,
		race.surfaceIndex,
		race.date,
		race.postTime,
		race.classification,
		race.classificationCode,
	); err != nil {
		return err
	}

	s2, err := tx.Prepare(`INSERT OR REPLACE INTO payout VALUES (?, ?, ?, ?, ?);`)
	if err != nil {
		return err
	}
	defer s2.Close()

	for i := 0; i < len(payouts); i++ {
		if _, err := s2.Exec(
			payouts[i].raceID,
			payouts[i].ticketType,
			payouts[i].draw,
			payouts[i].amount,
			payouts[i].popularity,
		); err != nil {
			return err
		}
	}

	s3, err := tx.Prepare(`INSERT OR REPLACE INTO result VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return err
	}
	defer s3.Close()

	for i := 0; i < len(results); i++ {
		if _, err := s3.Exec(
			results[i].raceID,
			results[i].orderOfFinish,
			results[i].bracket,
			results[i].draw,
			results[i].horseID,
			results[i].horse,
			results[i].sex,
			results[i].age,
			results[i].weight,
			results[i].jockeyID,
			results[i].jockey,
			results[i].time,
			results[i].timeSec,
			results[i].winningMargin,
			results[i].speedIndex,
			results[i].position,
			results[i].sectionalTime,
			results[i].odds,
			results[i].popularity,
			results[i].horseWeight,
			results[i].note,
			results[i].stable,
			results[i].trainerID,
			results[i].ownerID,
			results[i].earnings,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type race struct {
	id                 int
	name               string
	course             string
	number             int
	surface            string
	direction          string
	distance           int
	weather            string
	surfaceState       string
	surfaceIndex       sql.NullInt32
	date               string
	postTime           string
	classification     string
	classificationCode string
}

type payout struct {
	raceID     int
	ticketType string
	draw       string
	amount     float64
	popularity int
}

type result struct {
	raceID        int
	orderOfFinish string
	bracket       int
	draw          int
	horseID       int
	horse         string
	sex           string
	age           int
	weight        float64
	jockeyID      string
	jockey        string
	time          sql.NullString
	timeSec       sql.NullFloat64
	winningMargin string
	speedIndex    sql.NullInt32
	position      string
	sectionalTime sql.NullFloat64
	odds          float64
	popularity    int
	horseWeight   string
	note          string
	stable        string
	trainerID     string
	ownerID       string
	earnings      float64
}

func buildRaceRecord(id int, doc *html.Node) (*race, error) {
	record := &race{id: id}

	raceData := htmlquery.QuerySelector(doc, xpath.MustCompile(`//dl[`+util.xpathContains("@class", "racedata")+`]`))
	if raceData == nil {
		return nil, xerrors.New(`Missing race data from HTML`)
	}

	if dt, err := htmlquery.Query(raceData, "//dt"); err == nil {
		if i, err := strconv.Atoi(strings.TrimRight(util.htmlInnerText(dt), " R")); err == nil {
			record.number = i
		}
	} else {
		return nil, err
	}

	if h1, err := htmlquery.Query(raceData, "//h1"); err == nil {
		record.name = util.htmlInnerText(h1)
	} else {
		return nil, err
	}

	if span, err := htmlquery.Query(raceData, "//span"); err == nil {
		s := util.htmlInnerTextAndSplit(span, "/")

		// workaround for Stayers Stakes
		if strings.Contains(s[0], " 内2周") {
			s[0] = strings.Replace(s[0], " 内2周", "", -1)
		}

		r1 := regexp.MustCompile(`([^\d]+)([\d]+)m`)
		m1 := r1.FindAllStringSubmatch(s[0], -1)

		record.surface = string([]rune(m1[0][1])[:1])
		record.direction = string([]rune(m1[0][1])[1:])

		if i, err := strconv.Atoi(m1[0][2]); err == nil {
			record.distance = i
		}

		r2 := regexp.MustCompile(`.* \: (.+)`)

		record.weather = strings.Replace(strings.TrimSpace(s[1]), "天候 : ", "", -1)
		record.surfaceState = r2.ReplaceAllString(strings.TrimSpace(s[2]), "$1")
		record.postTime = strings.Replace(strings.TrimSpace(s[3]), "発走 : ", "", -1)
	} else {
		return nil, err
	}

	if p := htmlquery.QuerySelector(doc, xpath.MustCompile(`//p[`+util.xpathContains("@class", "smalltxt")+`]`)); p != nil {
		s := util.htmlInnerTextAndSplit(p, " ")
		t, _ := time.Parse("2006年1月2日", s[0])

		r := regexp.MustCompile(`.*(中京|中山|京都|函館|小倉|新潟|札幌|東京|福島|阪神).*`)

		record.course = r.ReplaceAllString(s[1], "$1")
		record.date = t.Format("2006-01-02")
		record.classification = s[2]
		record.classificationCode = determineClassificationCode(record.surface, record.distance, record.classification)
	} else {
		return nil, xerrors.New(`Missing p[@class="smalltxt"]`)
	}

	if td := htmlquery.QuerySelector(doc, xpath.MustCompile(`//table[@summary="馬場情報"]/tbody/tr/th[text()='馬場指数']/following-sibling::td`)); td != nil {
		t := util.htmlInnerText(td)

		if i, err := strconv.Atoi(t[:strings.Index(t, " ")]); err == nil {
			record.surfaceIndex.Scan(i)
		}
	}

	return record, nil
}

func buildPayoutRecords(id int, doc *html.Node) ([]*payout, error) {
	tr := htmlquery.QuerySelectorAll(doc, xpath.MustCompile(`//table[`+util.xpathContains("@class", "pay_table_01")+`]//tr`))
	if len(tr) == 0 {
		return nil, xerrors.New(`Missing payout table from HTML`)
	}

	var records []*payout

	for i := 0; i < len(tr); i++ {
		th := htmlquery.QuerySelector(tr[i], xpath.MustCompile(`//th`))
		td := htmlquery.QuerySelectorAll(tr[i], xpath.MustCompile(`//td`))

		draw := util.htmlSplitLineBreak(td[0])
		amount := util.htmlSplitLineBreak(td[1])
		popularity := util.htmlSplitLineBreak(td[2])

		for i := 0; i < len(draw); i++ {
			record := &payout{
				raceID:     id,
				ticketType: util.htmlInnerText(th),
				draw:       draw[i],
				amount:     util.parseFloat(amount[i]),
				popularity: util.atoi(popularity[i]),
			}

			records = append(records, record)
		}
	}

	return records, nil
}

func buildResultRecords(id int, doc *html.Node) ([]*result, error) {
	tr := htmlquery.QuerySelectorAll(doc, xpath.MustCompile(`//table[`+util.xpathContains("@class", "race_table_01")+`]//tr`))

	// first line is table header
	if len(tr) < 2 {
		return nil, xerrors.New(`race result not found, or is invalid`)
	}

	var records []*result

	for i := 1; i < len(tr); i++ {
		td := htmlquery.QuerySelectorAll(tr[i], xpath.MustCompile(`//td`))

		sex := string([]rune(util.htmlInnerText(td[4]))[:1])
		age, _ := strconv.Atoi(string([]rune(util.htmlInnerText(td[4]))[1:]))

		record := &result{
			raceID:        id,
			orderOfFinish: util.htmlInnerText(td[0]),
			bracket:       util.htmlInnerTextAsInt(td[1]),
			draw:          util.htmlInnerTextAsInt(td[2]),
			horseID:       util.atoi(util.htmlSelectHrefLastSegment(td[3])),
			horse:         util.htmlInnerText(td[3]),
			sex:           sex,
			age:           age,
			weight:        util.htmlInnerTextAsFloat(td[5]),
			jockeyID:      util.htmlSelectHrefLastSegment(td[6]),
			jockey:        util.htmlInnerText(td[6]),
			winningMargin: util.htmlInnerText(td[8]),
			position:      util.htmlInnerText(td[10]),
			odds:          util.htmlInnerTextAsFloat(td[12]),
			popularity:    util.htmlInnerTextAsInt(td[13]),
			horseWeight:   util.htmlInnerText(td[14]),
			note:          util.htmlInnerText(td[17]),
			stable:        string([]rune(util.htmlInnerText(td[18]))[1:2]),
			trainerID:     util.htmlSelectHrefLastSegment(td[18]),
			ownerID:       util.htmlSelectHrefLastSegment(td[19]),
			earnings:      util.htmlInnerTextAsFloat(td[20]),
		}

		if util.htmlInnerText(td[7]) != "" {
			record.time.Scan(util.htmlInnerText(td[7]))
			record.timeSec.Scan(util.parseFinishTime(util.htmlInnerText(td[7])))
		}
		if util.htmlInnerText(td[11]) != "" {
			record.sectionalTime.Scan(util.htmlInnerTextAsFloat(td[11]))
		}
		record.speedIndex.Scan(util.htmlInnerTextAsInt(td[9]))

		records = append(records, record)
	}

	return records, nil
}

const (
	// Turf, 1000 - 1300m
	classTurfSprintMaiden   = "TS0"
	classTurfSprintUntil3yo = "TS1"
	classTurfSprint3yoAndUp = "TS2"
	classTurfSprint         = "TS3"

	// Turf, 1301 - 1899m
	classTurfMileMaiden   = "TM0"
	classTurfMileUntil3yo = "TM1"
	classTurfMile3yoAndUp = "TM2"
	classTurfMile         = "TM3"

	// Turf, 1900 - 2100m
	classTurfIntermediateMaiden   = "TI0"
	classTurfIntermediateUntil3yo = "TI1"
	classTurfIntermediate3yoAndUp = "TI2"
	classTurfIntermediate         = "TI3"

	// Turf, 2101 - 2700m
	classTurfLongMaiden   = "TL0"
	classTurfLongUntil3yo = "TL1"
	classTurfLong3yoAndUp = "TL2"
	classTurfLong         = "TL3"

	// Turf, 2701m -
	classTurfExtendedMaiden   = "TE0"
	classTurfExtendedUntil3yo = "TE1"
	classTurfExtended3yoAndUp = "TE2"
	classTurfExtended         = "TE3"

	// Dirt, 1000 - 1300m
	classDirtSprintMaiden   = "DS0"
	classDirtSprintUntil3yo = "DS1"
	classDirtSprint3yoAndUp = "DS2"
	classDirtSprint         = "DS3"

	// Dirt, 1301 - 1899m
	classDirtMileMaiden   = "DM0"
	classDirtMileUntil3yo = "DM1"
	classDirtMile3yoAndUp = "DM2"
	classDirtMile         = "DM3"

	// Dirt, 1900 - 2100m
	classDirtIntermediateMaiden   = "DI0"
	classDirtIntermediateUntil3yo = "DI1"
	classDirtIntermediate3yoAndUp = "DI2"
	classDirtIntermediate         = "DI3"

	// Dirt, 2101 - 2700m
	classDirtLongMaiden   = "DL0"
	classDirtLongUntil3yo = "DL1"
	classDirtLong3yoAndUp = "DL2"
	classDirtLong         = "DL3"

	// steeplechase
	classSteeplechase ="S"
)

func determineClassificationCode(surface string, distance int, classification string) string {
	if surface == "障" {
		return classSteeplechase
	}

	if strings.Contains(classification, "オープン") ||
		strings.Contains(classification, "1600") ||
		strings.Contains(classification, "3勝") {
		if surface == "芝" {
			switch {
			case distance <= 1300:
				return classTurfSprint
			case 1301 <= distance && distance <= 1899:
				return classTurfMile
			case 1900 <= distance && distance <= 2100:
				return classTurfIntermediate
			case 2101 <= distance && distance <= 2700:
				return classTurfLong
			default:
				return classTurfExtended
			}
		} else {
			switch {
			case distance <= 1300:
				return classDirtSprint
			case 1301 <= distance && distance <= 1899:
				return classDirtMile
			case 1900 <= distance && distance <= 2100:
				return classDirtIntermediate
			default:
				return classDirtLong
			}
		}
	}

	if strings.Contains(classification, "3歳以上") ||
		strings.Contains(classification, "4歳以上") {
		if surface == "芝" {
			switch {
			case distance <= 1300:
				return classTurfSprint3yoAndUp
			case 1301 <= distance && distance <= 1899:
				return classTurfMile3yoAndUp
			case 1900 <= distance && distance <= 2100:
				return classTurfIntermediate3yoAndUp
			case 2101 <= distance && distance <= 2700:
				return classTurfLong3yoAndUp
			default:
				return classTurfExtended3yoAndUp
			}
		} else {
			switch {
			case distance <= 1300:
				return classDirtSprint3yoAndUp
			case 1301 <= distance && distance <= 1899:
				return classDirtMile3yoAndUp
			case 1900 <= distance && distance <= 2100:
				return classDirtIntermediate3yoAndUp
			default:
				return classDirtLong3yoAndUp
			}
		}
	}

	if strings.Contains(classification, "新馬") ||
		strings.Contains(classification, "未勝利") {
		if surface == "芝" {
			switch {
			case distance <= 1300:
				return classTurfSprintMaiden
			case 1301 <= distance && distance <= 1899:
				return classTurfMileMaiden
			case 1900 <= distance && distance <= 2100:
				return classTurfIntermediateMaiden
			case 2101 <= distance && distance <= 2700:
				return classTurfLongMaiden
			default:
				return classTurfExtendedMaiden
			}
		} else {
			switch {
			case distance <= 1300:
				return classDirtSprintMaiden
			case 1301 <= distance && distance <= 1899:
				return classDirtMileMaiden
			case 1900 <= distance && distance <= 2100:
				return classDirtIntermediateMaiden
			default:
				return classDirtLongMaiden
			}
		}
	}

	if surface == "芝" {
		switch {
		case distance <= 1300:
			return classTurfSprintUntil3yo
		case 1301 <= distance && distance <= 1899:
			return classTurfMileUntil3yo
		case 1900 <= distance && distance <= 2100:
			return classTurfIntermediateUntil3yo
		case 2101 <= distance && distance <= 2700:
			return classTurfLongUntil3yo
		default:
			return classTurfExtendedUntil3yo
		}
	} else {
		switch {
		case distance <= 1300:
			return classDirtSprintUntil3yo
		case 1301 <= distance && distance <= 1899:
			return classDirtMileUntil3yo
		case 1900 <= distance && distance <= 2100:
			return classDirtIntermediateUntil3yo
		default:
			return classDirtLongUntil3yo
		}
	}
}