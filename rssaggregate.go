package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/mmcdole/gofeed"
	"github.com/sljm12/gogeotext"
)

/*
SiteConfigItem - Config info for each topic and sites that the feeds come from
*/
type SiteConfigItem struct {
	Name       string   `json:"name"`
	Sites      []string `json:"sites"`
	OutputFile string   `json:"outputFile"`
}

/*
RSSItem to hold RSS data
*/
type RSSItem struct {
	URL             string   `json:"url"`
	Date            string   `json:"date"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Language        string   `json:"language"`
	TranslatedTitle string   `json:"translatedTitle"`
	Countries       []string `json:"countries"`
}

func (a RSSItem) getDate() string {
	return a.Date[0:10]
}

/*
AggregateFeed - After the aggregateFeed the results
*/
type AggregateFeed struct {
	SiteConfig   SiteConfigItem
	SortedDate   []string
	AggregateMap map[string][]RSSItem
}

func getFeedsFile(file string) []string {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return []string{}
	}

	r := []string{}
	for _, s := range strings.Split(string(content), "\n") {
		r = append(r, strings.TrimSpace(s))
	}

	return r
}

//Pipeline Components

/*
ExtractLanguage -  Extracts the language of the title
*/
func ExtractLanguage(rssItem *RSSItem) {
	lang := whatlanggo.Detect(rssItem.Title)
	rssItem.Language = lang.Lang.Iso6391()
}

/*
CreateGeoTextLocator - Create a closure that injects in a GeoTextLocator
*/
func CreateGeoTextLocator(geoTextLocator *gogeotext.GeoTextLocator) func(rssItem *RSSItem) {
	return func(rssItem *RSSItem) {
		if rssItem.Language != "zh" {
			locations := geoTextLocator.ExtractGeoLocation(rssItem.Title)
			countries := locations.Countries
			for _, v := range countries {
				//fmt.Println(v.CountryCode)
				rssItem.Countries = append(rssItem.Countries, v.CountryCode)
			}

			for _, v := range locations.Cities {
				if !checkStringInArray(v.CountryCode, &rssItem.Countries) {
					rssItem.Countries = append(rssItem.Countries, v.CountryCode)
				}
			}
		}
	}
}

func checkStringInArray(str string, ar *[]string) bool {
	for _, v := range *ar {
		if v == str {
			return true
		}
	}
	return false
}

func profileSpeed(f func(*RSSItem)) func(*RSSItem) {
	return func(rssItem *RSSItem) {
		start := time.Now()
		f(rssItem)
		t := time.Now()
		fmt.Println(t.Sub(start))
	}
}

func readConfigFile(fn string) ([]SiteConfigItem, error) {
	content, err := ioutil.ReadFile(fn)
	var items []SiteConfigItem
	if err == nil {

		jerr := json.Unmarshal(content, &items)
		if jerr != nil {
			fmt.Println(jerr)
			fmt.Println("Failed to load")
		}
		return items, nil
	} else {
		return []SiteConfigItem{}, err
	}
}

func getFeed(name string, url chan string, mutex *sync.Mutex, wg *sync.WaitGroup, sharedArray *map[string][]RSSItem) {
	defer wg.Done()

	fp := gofeed.NewParser()
	for link := range url {
		feed, _ := fp.ParseURL(strings.TrimSpace(link))

		if feed != nil {
			mutex.Lock()
			for _, v := range feed.Items {
				postDate := ""
				if v.Published != "" {
					postDate = v.PublishedParsed.Format(time.RFC3339)
				} else if v.Updated != "" {
					postDate = v.UpdatedParsed.Format(time.RFC3339)
				} else {
					postDate = ""
				}
				//langInfo := whatlanggo.Detect(v.Title)

				item := RSSItem{Title: v.Title, Content: v.Content, Date: postDate, URL: v.Link}

				updateMap(sharedArray, item)
			}
			mutex.Unlock()
		}
	}
}

func updateMap(sharedMap *map[string][]RSSItem, rssItem RSSItem) {

	date := rssItem.Date[0:10]
	_, ok := (*sharedMap)[date]
	if ok {
		arr := (*sharedMap)[date]
		url := rssItem.URL

		if isUrlinArr(arr, url) == false {
			arr := append(arr, rssItem)
			(*sharedMap)[date] = arr
		}

	} else {
		arr := []RSSItem{rssItem}
		(*sharedMap)[date] = arr
	}
}

func isUrlinArr(arr []RSSItem, url string) bool {
	for _, v := range arr {
		if v.URL == url {
			return true
		}
	}
	return false
}

func compareStrings(date1, date2 string) bool {
	return date1 < date2
}

func aggregateFeed(links []string, collectors int) (map[string][]RSSItem, []string) {
	sharedData := make(map[string][]RSSItem)
	var mux sync.Mutex
	var wg sync.WaitGroup

	url := make(chan string)

	for a := 0; a < collectors; a++ {
		wg.Add(1)
		go getFeed(strconv.Itoa(a), url, &mux, &wg, &sharedData)
	}

	for _, v := range links {
		url <- v
	}
	close(url)
	wg.Wait()
	var sortedDate []string

	for k := range sharedData {
		sortedDate = append(sortedDate, k)
	}

	sort.Slice(sortedDate, func(i, j int) bool { return sortedDate[i] < sortedDate[j] })

	return sharedData, sortedDate
}

func processRSSItem(itemchan chan []RSSItem, aggregateData *map[string][]RSSItem) {
	for v := range itemchan {
		for _, r := range v {
			updateMap(aggregateData, r)
		}
	}
}

/*
writeData - outputs aggregateFeed to a json file
*/
func writeData(dir string, aggregateFeed AggregateFeed) {
	j, err := json.MarshalIndent(aggregateFeed, "", "\t")
	if err != nil {
		panic(err)
	} else {
		filename := path.Join(dir, aggregateFeed.SiteConfig.Name+".json")
		err := ioutil.WriteFile(filename, j, 0644)
		if err != nil {
			panic(err)
		}
	}
}

/*
processPipeline - run the RSSItem in AggregateFeed through the commands in the pipeline
*/
func processPipeline(aggregateFeed *AggregateFeed, pipeline []func(rssItem *RSSItem), numRoutines int) *AggregateFeed {
	var wg sync.WaitGroup
	var owg sync.WaitGroup
	channel := make(chan RSSItem)
	outChannel := make(chan RSSItem)
	var resultAggregateFeed AggregateFeed
	resultAggregateFeed.AggregateMap = make(map[string][]RSSItem)
	resultAggregateFeed.SortedDate = aggregateFeed.SortedDate
	resultAggregateFeed.SiteConfig = aggregateFeed.SiteConfig

	owg.Add(1)
	go printOutChannel(outChannel, &owg, &resultAggregateFeed)

	for a := 0; a < numRoutines; a++ {
		wg.Add(1)
		go process(strconv.Itoa(a), channel, outChannel, &wg, pipeline)
	}

	for _, d := range aggregateFeed.SortedDate {
		for _, r := range aggregateFeed.AggregateMap[d] {
			channel <- r
		}
	}

	close(channel)
	wg.Wait()
	close(outChannel) //We close the outChannel when all the process go routines are done

	owg.Wait() //Wait for the out routine to be done
	return &resultAggregateFeed
}

func process(num string, channel chan RSSItem, outChannel chan RSSItem, wg *sync.WaitGroup, pipeline []func(rssItem *RSSItem)) {
	defer wg.Done()
	for rssItem := range channel {
		fmt.Println("Processing " + num)
		for _, p := range pipeline {
			p(&rssItem)
		}
		outChannel <- rssItem
	}

}

/*
Aggregate the data from the pipeline into one feed again after processing
*/
func printOutChannel(channel chan RSSItem, wg *sync.WaitGroup, aggregateFeed *AggregateFeed) {
	defer wg.Done()
	for rss := range channel {
		fmt.Println("Out " + rss.Language)
		(*aggregateFeed).AggregateMap[rss.getDate()] = append((*aggregateFeed).AggregateMap[rss.getDate()], rss)
	}
}

func test() *AggregateFeed {
	gtl := gogeotext.CreateDefaultGeoTextLocator("./alternateName.csv", "./cities500.txt", "./default_city.csv")
	gtlExtract := CreateGeoTextLocator(&gtl)

	pipeline := []func(rssItem *RSSItem){ExtractLanguage, profileSpeed(gtlExtract)}

	var a []AggregateFeed

	var one AggregateFeed
	one.AggregateMap = make(map[string][]RSSItem)
	one.AggregateMap["one"] = []RSSItem{}
	one.SortedDate = append(one.SortedDate, "one")
	numOfRecords, _ := strconv.Atoi(os.Args[1])
	threads, _ := strconv.Atoi(os.Args[2])
	//numOfRecords := 1
	//threads := 1
	for c := 0; c < numOfRecords; c++ {
		i := RSSItem{Title: "San Diego, Mexico is a great place.", Date: "2016-02-02T12:00:00"}
		one.AggregateMap["one"] = append(one.AggregateMap["one"], i)
	}

	a = append(a, one)
	start := time.Now()
	result := processPipeline(&a[0], pipeline, threads)
	end := time.Now()
	fmt.Println(end.Sub(start))
	fmt.Println(result.AggregateMap["2016-02-02"][0].Countries)
	return result
}

/*
trimAggregateFeed - Reduce the number of days in aggreagateFeed
*/
func trimAggregateFeed(feeds *AggregateFeed, numDaysBack int) AggregateFeed {
	tn := time.Now()
	sd := time.Date(tn.Year(), tn.Month(), tn.Day(), 0, 0, 0, 0, tn.Location()).AddDate(0, 0, 1)
	minimiumDate := sd.AddDate(0, 0, numDaysBack*-1)
	fmt.Println(minimiumDate)
	newSortedDate := []string{}
	for _, d := range feeds.SortedDate {
		rfcDate := d + "T00:00:00Z"
		idate, err := time.Parse(time.RFC3339, rfcDate)
		if err == nil {
			if idate.Before(minimiumDate) {
				delete((*feeds).AggregateMap, d)
			} else {
				newSortedDate = append(newSortedDate, d)
			}
		}
	}
	(*feeds).SortedDate = newSortedDate
	return *feeds
}

func main() {
	/*
		var translater translate.IbmTranslator

		translater.Setup(os.Getenv("ibm_api"), os.Getenv("ibm_url"), "2018-05-01")
		fmt.Println(translater.Translate("我是一个好人", "zh", "en"))
	*/
	/*
		a := test()
		fmt.Println("End")
		fmt.Println(a.AggregateMap["2016-02-02"][0].countries)
	*/
	items, err := readConfigFile("./config.json")
	if err != nil {
		panic(err)
	}

	start := time.Now()
	var aggregateResults []AggregateFeed

	for i, v := range items {
		aggregateData, sortedDates := aggregateFeed(v.Sites, 4)
		fmt.Println(i, v.Name)
		aggregateFeed := AggregateFeed{SiteConfig: v, SortedDate: sortedDates, AggregateMap: aggregateData}
		aggregateResults = append(aggregateResults, aggregateFeed)
	}

	//Trim the dates
	for i, feed := range aggregateResults {
		r := trimAggregateFeed(&feed, 2)
		fmt.Println(len(feed.SortedDate) == 2)
		aggregateResults[i] = r
	}
	//Pipeline
	gtl := gogeotext.CreateDefaultGeoTextLocator("./alternateName.csv", "./cities500.txt", "./default_city.csv")
	gtlExtract := CreateGeoTextLocator(&gtl)

	pipeline := []func(rssItem *RSSItem){ExtractLanguage, gtlExtract}

	for i, r := range aggregateResults {
		ans := processPipeline(&r, pipeline, 3)
		aggregateResults[i] = *ans
	}

	elasped := time.Now().Sub(start)
	fmt.Println(elasped)

	totalRows := 0
	for _, r := range aggregateResults {
		fmt.Println(r.SiteConfig.Name)
		for _, d := range r.SortedDate {
			fmt.Println(d, len(r.AggregateMap[d]))
			/*
				for _, v := range r.AggregateMap[d] {
					item := v
					fmt.Println(item.title, item.language)
				}
			*/
			totalRows = totalRows + len(r.AggregateMap[d])
		}
		writeData("./data", r)
	}
	fmt.Printf("Total Rows %d", totalRows)

}
