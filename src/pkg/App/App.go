package a

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vault-thirteen/Forum-Crawler/src/models"
	"github.com/vault-thirteen/Forum-Crawler/src/pkg/CLIArguments"
	"github.com/vault-thirteen/Forum-Crawler/src/pkg/db"
	htmldom "github.com/vault-thirteen/HTML-DOM"
	ae "github.com/vault-thirteen/auxie/errors"
	"github.com/vault-thirteen/auxie/number"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
)

const (
	AttributeId   = "id"
	AttributeHref = "href"
)

const (
	ErrDomNodeIsNotFound       = "DOM node is not found"
	ErrTrWithIdIsNotFound      = "<tr> with ID is not found"
	ErrAWithIdIsNotFound       = "<a> with ID is not found"
	ErrNoNumberInId            = "no number in ID: %v"
	ErrIdAttributeIsNotFound   = "ID attribute is not found"
	ErrHrefAttributeIsNotFound = "href attribute is not found"
	ErrTopicIdMismatch         = "topic ID mismatch: %v vs %v"
	ErrHrefMismatch            = "href mismatch: %v vs %v"
	ErrNoPageNumbers           = "no page numbers"
	ErrCsvSyntax               = "CSV syntax error: %v"
	ErrUnsupportedEncoding     = "unsupported encoding: %v"
)

const (
	PageNumberAllPages = 0
	TagWbr             = `<wbr/>`
)

type App struct {
	// Settings.
	CLIArgs  *cli.Arguments
	Settings *models.Settings

	// Internal Structures.
	Db *db.DB

	// Various Data.
	Forums []*models.Forum
}

func NewApp(cliArgs *cli.Arguments) (app *App, err error) {
	app = &App{
		CLIArgs: cliArgs,
	}

	app.Settings, err = app.getSettings(cliArgs.SettingsFile)
	if err != nil {
		return nil, err
	}

	app.Db, err = db.NewDB(app.Settings.Database)
	if err != nil {
		return nil, err
	}

	switch cliArgs.Action {
	case cli.ActionInit:
		switch cliArgs.Object {
		case cli.ObjectForums: // init forums.
			_, err = app.initForums()
			if err != nil {
				return nil, err
			}

		case cli.ObjectForumTopics: // init forum_topics.
			err = app.initForumTopics()
			if err != nil {
				return nil, err
			}

		case cli.ObjectAllTopics: // init all_topics.
			err = app.initAllTopics()
			if err != nil {
				return nil, err
			}

		default: // init *.
			return nil, fmt.Errorf(cli.ErrUnsupportedObject, cliArgs.Object)
		}

	case cli.ActionRefresh:
		switch cliArgs.Object {

		case cli.ObjectAllTopics: // refresh all_topics.
			err = app.refreshAllTopics()
			if err != nil {
				return nil, err
			}

		default: // refresh *.
			return nil, fmt.Errorf(cli.ErrUnsupportedObject, cliArgs.Object)
		}

	default: // * *.
		return nil, fmt.Errorf(cli.ErrUnknownAction, cliArgs.Action)
	}

	return app, nil
}

func (a *App) getSettings(file string) (s *models.Settings, err error) {
	var buf []byte
	buf, err = os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	s = &models.Settings{}
	err = json.Unmarshal(buf, s)
	if err != nil {
		return nil, err
	}

	// Additional settings.
	s.Database.TemporaryFolder = s.TemporaryFolder

	return s, nil
}

func (a *App) Close() (err error) {
	err = a.Db.Close()
	if err != nil {
		return err
	}

	return nil
}

// initForums reads forums from a file and saves them into the database.
func (a *App) initForums() (forums []*models.Forum, err error) {
	log.Println("Initializing list of forums")

	forums, err = a.getForums(a.Settings.ForumsFile)
	if err != nil {
		return nil, err
	}

	return forums, a.saveForums(forums)
}

// getForums reads forums from a file.
func (a *App) getForums(forumsFile string) (forums []*models.Forum, err error) {
	var f *os.File
	f, err = os.Open(forumsFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		derr := f.Close()
		if derr != nil {
			err = ae.Combine(err, derr)
		}
	}()

	csvReader := csv.NewReader(f)
	var records [][]string
	records, err = csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	var forum *models.Forum
	forums = make([]*models.Forum, 0, len(records))
	for _, rec := range records {
		if len(rec) != 2 {
			return nil, fmt.Errorf(ErrCsvSyntax, rec)
		}

		forum = &models.Forum{
			Name: rec[1],
		}
		forum.ID, err = number.ParseUint(rec[0])
		if err != nil {
			return nil, err
		}

		forums = append(forums, forum)
	}

	return forums, nil
}

// saveForums saves forums into the database.
func (a *App) saveForums(forums []*models.Forum) (err error) {
	for _, f := range forums {
		err = a.Db.SaveForum(f)
		if err != nil {
			return err
		}
	}

	return nil
}

// initForumTopics reads forum's topics from internet and saves them into the
// database.
// If 'pageNumber' is 0, all pages will be scanned, otherwise only a single page
// will be scanned for topics.
func (a *App) initForumTopics() (err error) {
	log.Println("Initializing forum's topics")

	var forumId uint
	forumId, err = a.CLIArgs.GetForumId()
	if err != nil {
		return err
	}

	var pageNumber uint
	pageNumber, err = a.CLIArgs.GetForumPage()
	if err != nil {
		return err
	}

	var topics map[uint]*models.Topic
	topics, err = a.getForumTopics(forumId, pageNumber)
	if err != nil {
		return err
	}

	return a.saveTopics(forumId, topics)
}

// initAllTopics reads topics of all forums from internet and saves them into
// the database.
// The 'startForumId' is used to resume updates on a selected forum. If it is
// set to zero, all forums are scanned without resuming.
func (a *App) initAllTopics() (err error) {
	a.Forums, err = a.initForums()
	if err != nil {
		return err
	}

	var startForumId uint
	startForumId, err = a.CLIArgs.GetStartForumId()
	if err != nil {
		return err
	}

	var startForumIsFound bool
	if startForumId == 0 {
		startForumIsFound = true
	}

	log.Println("Initializing all topics")

	var topics map[uint]*models.Topic

	for _, forum := range a.Forums {
		if !startForumIsFound {
			// Skip some forums until we find the one for a good start.
			if forum.ID != startForumId {
				continue
			} else {
				startForumIsFound = true
			}
		}

		topics, err = a.getForumTopics(forum.ID, PageNumberAllPages)
		if err != nil {
			return err
		}

		err = a.saveTopics(forum.ID, topics)
		if err != nil {
			return err
		}
	}

	return nil
}

// refreshAllTopics reads topics from first N pages of all forums from internet
// and saves new topics into the database. Existing topics are not updated.
func (a *App) refreshAllTopics() (err error) {
	a.Forums, err = a.initForums()
	if err != nil {
		return err
	}

	var firstPagesCount uint
	firstPagesCount, err = a.CLIArgs.GetFirstPages()
	if err != nil {
		return err
	}

	log.Println(fmt.Sprintf("Refreshing all topics. FP=%v.", firstPagesCount))

	var topics map[uint]*models.Topic

	for _, forum := range a.Forums {
		topics, err = a.getForumTopicsFromFirstPages(forum.ID, firstPagesCount)
		if err != nil {
			return err
		}

		err = a.saveNewTopics(forum.ID, topics)
		if err != nil {
			return err
		}
	}

	return nil
}

// getForumTopics fetches forum's topics from internet.
// If pageNumber is 0, all pages will be scanned, otherwise only a single page
// will be scanned for topics.
func (a *App) getForumTopics(forumId uint, pageNumber uint) (uniqueTopics map[uint]*models.Topic, err error) {
	var pageSrc []byte
	var topics []*models.Topic
	uniqueTopics = make(map[uint]*models.Topic)

	// Single page fetch.
	if pageNumber != 0 {
		fmt.Println(fmt.Sprintf("Forum ID=%v: [%v]", forumId, pageNumber))

		pageSrc, err = a.getForumPage(forumId, (pageNumber-1)*a.Settings.TopicsPerPage)
		if err != nil {
			return nil, err
		}

		topics, err = a.findForumTopics(forumId, pageSrc)
		if err != nil {
			return nil, err
		}

		for _, topic := range topics {
			uniqueTopics[topic.Id] = topic
		}
		return uniqueTopics, nil
	}

	// Fetch all the pages.
	fmt.Printf("Forum ID=%v: ", forumId)

	pageSrc, err = a.getForumPage(forumId, 0)
	if err != nil {
		return nil, err
	}

	var pageCount uint
	pageCount, err = a.findForumPagesCount(forumId, pageSrc)
	if err != nil {
		return nil, err
	}

	a.sleepBetweenTopicPages()

	for pageNum := uint(1); pageNum <= pageCount; pageNum++ {
		fmt.Printf("[%v] ", pageNum)

		pageSrc, err = a.getForumPage(forumId, (pageNum-1)*a.Settings.TopicsPerPage)
		if err != nil {
			return nil, err
		}

		topics, err = a.findForumTopics(forumId, pageSrc)
		if err != nil {
			return nil, err
		}

		var topicExists bool
		for _, topic := range topics {
			_, topicExists = uniqueTopics[topic.Id]
			if topicExists {
				continue
			}
			uniqueTopics[topic.Id] = topic
		}

		a.sleepBetweenTopicPages()
	}

	fmt.Println()
	return uniqueTopics, nil
}

// getForumTopicsFromFirstPages fetches forum's topics from N first pages from
// internet.
func (a *App) getForumTopicsFromFirstPages(forumId uint, pagesCount uint) (uniqueTopics map[uint]*models.Topic, err error) {
	var pageSrc []byte
	var topics []*models.Topic
	uniqueTopics = make(map[uint]*models.Topic)

	// Fetch first N pages.
	fmt.Printf("Forum ID=%v: ", forumId)

	for pageNum := uint(1); pageNum <= pagesCount; pageNum++ {
		fmt.Printf("[%v] ", pageNum)

		pageSrc, err = a.getForumPage(forumId, (pageNum-1)*a.Settings.TopicsPerPage)
		if err != nil {
			return nil, err
		}

		topics, err = a.findForumTopics(forumId, pageSrc)
		if err != nil {
			return nil, err
		}

		var topicExists bool
		for _, topic := range topics {
			_, topicExists = uniqueTopics[topic.Id]
			if topicExists {
				continue
			}
			uniqueTopics[topic.Id] = topic
		}

		a.sleepBetweenTopicPages()
	}

	fmt.Println()
	return uniqueTopics, nil
}

// saveTopics saves topics into the database.
func (a *App) saveTopics(forumId uint, topics map[uint]*models.Topic) (err error) {
	if len(topics) < db.BulkThresholdCount {
		for _, topic := range topics {
			err = a.Db.SaveTopic(topic)
			if err != nil {
				return err
			}
		}
	} else {
		err = a.Db.SaveTopics(forumId, topics)
		if err != nil {
			return err
		}
	}

	return nil
}

// saveNewTopics saves [only] new topics into the database.
func (a *App) saveNewTopics(forumId uint, topics map[uint]*models.Topic) (err error) {
	var isTopicArchived = false
	if forumId == a.Settings.ArchivedTopicsForumId {
		isTopicArchived = true
	}

	if isTopicArchived {
		fmt.Print("Refreshing archived topics: ")
	} else {
		fmt.Print("Refreshing topics: ")
	}

	for _, topic := range topics {
		fmt.Printf("[%v] ", topic.Id)
		err = a.Db.SaveNewTopic(topic, isTopicArchived)
		if err != nil {
			return err
		}
	}
	fmt.Println()

	return nil
}

// getForumPage fetches source code of a specified forum page.
func (a *App) getForumPage(forumId uint, startItemIdx uint) (pageContents []byte, err error) {
	url := fmt.Sprintf(a.Settings.ForumUrlFormat, forumId, startItemIdx)

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	cookie := a.Settings.Cookie
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", a.Settings.UserAgent)

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		derr := resp.Body.Close()
		if derr != nil {
			err = ae.Combine(err, derr)
		}
	}()

	pageContents, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return a.decodeBytes(pageContents)
}

func (a *App) decodeBytes(dataInput []byte) (utfOutput []byte, err error) {
	switch a.Settings.PageEncoding {
	case models.PageEncoding_UTF8:
		return dataInput, nil
	case models.PageEncoding_Windows1251:
		return a.decodeWindows1251(dataInput)
	default:
		return nil, fmt.Errorf(ErrUnsupportedEncoding, a.Settings.PageEncoding)
	}
}

func (a *App) decodeWindows1251(cp1251Input []byte) (utfOutput []byte, err error) {
	return charmap.Windows1251.NewDecoder().Bytes(cp1251Input)
}

// findForumPagesCount searches for the count of pages in the source code of a
// page.
func (a *App) findForumPagesCount(forumId uint, pageContents []byte) (pageCount uint, err error) {
	var domNode *html.Node
	domNode, err = html.Parse(strings.NewReader(string(pageContents)))
	if err != nil {
		return 0, err
	}
	if domNode == nil {
		return 0, errors.New(ErrDomNodeIsNotFound)
	}

	var node = htmldom.GetChildNodeByTag(domNode, htmldom.TagHtml) // -> <doctype>
	node = htmldom.GetSiblingNodeByTag(node, htmldom.TagHtml)      // -> <html>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagHead)        // -> <head>
	node = htmldom.GetSiblingNodeByTag(node, htmldom.TagBody)      // -> <body>
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagDiv, "body_container")
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagDiv, "page_container")
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagDiv, "page_content")
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTable)                   // <table>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTbody)                   // <tbody>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTr)                      // <tr>
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagTd, "main_content") // <td id="main_content">
	node = htmldom.GetChildNodeByTag(node, htmldom.TagDiv)                     // <div id="main_content_wrap">
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTable)                   // <table>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTbody)                   // <tbody>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTr)                      // <tr>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTd)                      // <td>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagH1)                      // <h1>
	node = htmldom.GetSiblingNodeByTag(node, htmldom.TagP)                     // -> <p>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagB)                       // -> first <b>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagB)                       // -> second <b>

	pageNumbers := make([]uint, 0)
	var pageNumber uint
	node = htmldom.GetSiblingNodeByTag(node, htmldom.TagA) // first <a> with 'pg' class.

	for {
		pageNumber, err = a.getHtmlNodeInnerHtmlUint(node)
		if err != nil {
			break
		}
		pageNumbers = append(pageNumbers, pageNumber)

		node = htmldom.GetSiblingNodeByTag(node, htmldom.TagA) // next <a> with 'pg' class.
	}

	if len(pageNumbers) == 0 {
		return 0, errors.New(ErrNoPageNumbers)
	}
	pageNumber = pageNumbers[len(pageNumbers)-1]

	return pageNumber, nil
}

// findForumTopics searches for topics in the source code of a forum page.
func (a *App) findForumTopics(forumId uint, pageContents []byte) (topics []*models.Topic, err error) {
	var domNode *html.Node
	domNode, err = html.Parse(strings.NewReader(string(pageContents)))
	if err != nil {
		return nil, err
	}
	if domNode == nil {
		return nil, errors.New(ErrDomNodeIsNotFound)
	}

	var node = htmldom.GetChildNodeByTag(domNode, htmldom.TagHtml) // -> <doctype>
	node = htmldom.GetSiblingNodeByTag(node, htmldom.TagHtml)      // -> <html>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagHead)        // -> <head>
	node = htmldom.GetSiblingNodeByTag(node, htmldom.TagBody)      // -> <body>
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagDiv, "body_container")
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagDiv, "page_container")
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagDiv, "page_content")
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTable)                            // <table>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTbody)                            // <tbody>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTr)                               // <tr>
	node = htmldom.GetChildNodeByTagAndId(node, htmldom.TagTd, "main_content")          // <td id="main_content">
	node = htmldom.GetChildNodeByTag(node, htmldom.TagDiv)                              // <div id="main_content_wrap">
	node = htmldom.GetChildNodeByTagAndClass(node, htmldom.TagTable, "forumline forum") // <table class="forumline forum">
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTbody)                            // <tbody>
	node = htmldom.GetChildNodeByTag(node, htmldom.TagTr)                               // <tr>

	for {
		if htmldom.NodeHasAttribute(node, AttributeId) {
			break
		}

		node = htmldom.GetSiblingNodeByTag(node, htmldom.TagTr) // next <tr>
		if node == nil {
			a.printNodeDebugInfo(node)
			return nil, errors.New(ErrTrWithIdIsNotFound)
		}
	}

	topics = make([]*models.Topic, 0)
	var topic *models.Topic
	for {
		topic = &models.Topic{
			ForumId: forumId,
		}

		var id string
		var ok bool
		id, ok = htmldom.GetNodeAttributeValue(node, AttributeId)
		if ok {
			topic.Id, err = a.getNumberFromId(id)
			if err != nil {
				return nil, err
			}

			topic.Name, err = a.getTopicName(node, topic.Id)
			if err != nil {
				return nil, err
			}

			topics = append(topics, topic)
		}

		// Next <tr>.
		node = htmldom.GetSiblingNodeByTag(node, htmldom.TagTr)
		if node == nil {
			break
		}
	}

	return topics, nil
}

func (a *App) getNumberFromId(idStr string) (n uint, err error) {
	parts := strings.Split(idStr, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf(ErrNoNumberInId, idStr)
	}

	return number.ParseUint(parts[1])
}

// getTopicName searches for the topic name in a piece of an HTML code.
// N.B. 'trNode' argument must be preserved, i.e. it is read-only !
func (a *App) getTopicName(trNode *html.Node, topicId uint) (topicName string, err error) {
	n := htmldom.GetChildNodeByTagAndClass(trNode, htmldom.TagTd, "tt")  // <td style="padding: 3px 5px 3px 3px;" class="tt">
	n = htmldom.GetChildNodeByTagAndClass(n, htmldom.TagDiv, "torTopic") // <div class="torTopic">
	n = htmldom.GetChildNodeByTag(n, htmldom.TagA)                       // <a>

	for {
		if htmldom.NodeHasAttribute(n, AttributeId) {
			break
		}

		n = htmldom.GetSiblingNodeByTag(n, htmldom.TagA) // next <a>
		if n == nil {
			return "", errors.New(ErrAWithIdIsNotFound)
		}
	}

	// Integrity Check.
	idStr, ok := htmldom.GetNodeAttributeValue(n, AttributeId)
	if !ok {
		return "", errors.New(ErrIdAttributeIsNotFound)
	}
	var id uint
	id, err = a.getNumberFromId(idStr)
	if err != nil {
		return "", err
	}
	if id != topicId {
		return "", fmt.Errorf(ErrTopicIdMismatch, topicId, id)
	}

	var href string
	href, ok = htmldom.GetNodeAttributeValue(n, AttributeHref)
	if !ok {
		return "", errors.New(ErrHrefAttributeIsNotFound)
	}
	if !strings.Contains(href, strconv.FormatUint(uint64(topicId), 10)) {
		return "", fmt.Errorf(ErrHrefMismatch, topicId, href)
	}

	topicName, err = htmldom.GetInnerHtml(n)
	if err != nil {
		return "", err
	}

	return clearName(topicName), nil
}

func (a *App) getHtmlNodeInnerHtmlUint(n *html.Node) (num uint, err error) {
	var text string
	text, err = htmldom.GetInnerHtml(n)
	if err != nil {
		return 0, err
	}

	return number.ParseUint(text)
}

func (a *App) printNodeDebugInfo(n *html.Node) {
	text, err := htmldom.GetOuterHtml(n)
	if err != nil {
		log.Println(fmt.Sprintf("Node: %+v, Error: %v.", n, err))
	} else {
		log.Println(text)
	}
}

func (a *App) sleepBetweenTopicPages() {
	time.Sleep(time.Duration(float64(time.Second) * a.Settings.ForumTopicsPageDelaySec))
}

func clearName(dirtyName string) (cleanName string) {
	return remove4BytedRunes(html.UnescapeString(strings.ReplaceAll(dirtyName, TagWbr, "")))
}

// Unfortunately, UTF-8 encoding in MySQL supports runes having 3 bytes maximum.
// This is why the limit for indices is 3072 bytes (1024 x 3).
// Support for full-length (4 byte) UTF-8 is planned for future.
// Until that time, we are removing unsupported symbols from strings.
func remove4BytedRunes(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	for _, r := range runes {
		if getRuneSize(r) < 4 {
			sb.WriteRune(r)
		} else {
			log.Println(fmt.Sprintf("unsupported rune was removed: %v", string(r)))
		}
	}
	return sb.String()
}

func getRuneSize(r rune) int {
	return len([]byte(string(r)))
}
