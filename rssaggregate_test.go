package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/sljm12/gogeotext"
)

func TestPipeline(t *testing.T) {
	gtl := gogeotext.CreateDefaultGeoTextLocator("./alternateName.csv", "./cities500.txt", "./default_city.csv")
	gtlExtract := CreateGeoTextLocator(&gtl)

	pipeline := []func(rssItem *RSSItem){ExtractLanguage, profileSpeed(gtlExtract)}
	//Test extraction
	var rssItem RSSItem
	rssItem.Title = "San Diego, Mexico is a great place!"
	for _, v := range pipeline {
		v(&rssItem)
	}
	fmt.Println(rssItem)

	if rssItem.Countries[0] != "MX" {
		t.Error("Wrong Country")
	}

	//Test if not chinese then extract
	rssItem = RSSItem{Title: "中國積極投資新基建誰是最大贏家？ - 萬里富 - 華富財經"}

	for _, v := range pipeline {
		v(&rssItem)
	}
	fmt.Println(rssItem)

}

func TestPipelineSpeed(t *testing.T) {
	gtl := gogeotext.CreateDefaultGeoTextLocator("./alternateName.csv", "./cities500.txt", "./default_city.csv")
	gtlExtract := CreateGeoTextLocator(&gtl)

	pipeline := []func(rssItem *RSSItem){ExtractLanguage, profileSpeed(gtlExtract)}

	var a []AggregateFeed

	var one AggregateFeed
	one.AggregateMap = make(map[string][]RSSItem)
	one.AggregateMap["one"] = []RSSItem{}

	for c := 0; c < 10; c++ {
		i := RSSItem{Title: "San Diego, Mexico is a great place."}
		one.AggregateMap["one"] = append(one.AggregateMap["one"], i)
	}

	a = append(a, one)
	processPipeline(&a[0], pipeline, 1)
	firstElement := a[0].AggregateMap["one"][0]
	if firstElement.Language == "" {
		t.Error("No language found")
	}

	if firstElement.Countries[0] != "MX" {
		t.Error("Country Wrong")
	}
}

func TestRSSItemDate(t *testing.T) {
	a := RSSItem{Date: "2015-02-02T12:45:66"}
	if a.getDate() != "2015-02-02" {
		t.Error("Date Error")
	}
}

func TestUnmarshalTie(t *testing.T) {
	td := "2006-01-02T15:04:05Z"
	ti, error := time.Parse(time.RFC3339, td)
	if error != nil {
		t.Error(error)
	}
	fmt.Println(ti)
}

func TestTrimAggregateFeed(t *testing.T) {
	var a AggregateFeed
	a.AggregateMap = make(map[string][]RSSItem)
	current := time.Now()
	current = time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, current.Location())

	for c := 0; c < 10; c++ {
		n := current.AddDate(0, 0, c*-1)
		ds := n.Format(time.RFC3339)[0:10]
		a.AggregateMap[ds] = []RSSItem{}
		a.SortedDate = append(a.SortedDate, ds)
	}

	fmt.Println(a.AggregateMap)
	trimAggregateFeed(&a, 5)
	fmt.Println(a.AggregateMap)
	fmt.Println(a.SortedDate)
	if len(a.AggregateMap) != 5 {
		t.Error("Expected 5")
	}

	if len(a.SortedDate) != 5 {
		t.Error("Expected 5")
	}

	for _, c := range a.SortedDate {
		ans := a.AggregateMap[c]
		if ans == nil {
			t.Error("Suppose to find the key in map")
		}
	}
}

func TestCheckStringInArray(t *testing.T) {
	arr := []string{"US", "SG", "UK"}
	if !checkStringInArray("SG", &arr) {
		t.Error("SG suppose be true")
	}

	if checkStringInArray("IN", &arr) {
		t.Error("IN suppose to be false")
	}
}
