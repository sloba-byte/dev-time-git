package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"
	t "time"

	"github.com/google/go-github/v54/github"
) // with go modules enabled (GO111MODULE=on or outside GOPATH)

type CommitInfo struct {
	Title      string
	StartWork  t.Time
	FinishWork t.Time
	PrUrl      string
	Init       bool
}

type WorkInterval struct {
	Start t.Time
	End   t.Time
	Infos []CommitInfo
}

type ChartInto struct {
	Date  t.Time
	Value int
}

type WorkDay struct {
	Sleep int
	Work  int
	Waste int

	WorkToWaste float64

	DayIn15Minuts []ChartInto
}

func main() {
	client := github.NewClient(nil)

	owner := "petrovicdarko1234"
	repo := "zadaci"

	ctx := context.Background()

	prOpts := github.PullRequestListOptions{
		//State: "all",
	}
	prs, _, err := client.PullRequests.List(ctx, owner, repo, &prOpts)
	if err != nil {
		log.Fatal(err)
	}

	timeZoneDiff := time.Hour * 2

	//TODO remove this for prod
	//prs = prs[1:2]

	var commitDates []CommitInfo
	for _, p := range prs {
		commits, _, err := client.PullRequests.ListCommits(ctx, owner, repo, *p.Number, nil)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(*p.Title)

		for i := 0; i < len(commits); i++ {
			date := commits[i].Commit.Author.Date.Time.Add(timeZoneDiff)
			startWork := date.Add(-(time.Hour / 2))
			if i == 0 {
				startWork = date.Add(-(2 * time.Hour))
			}
			commitDates = append(commitDates, CommitInfo{
				Title:      *p.Title,
				FinishWork: date,
				PrUrl:      *p.URL,
				Init:       i == 0,
				StartWork:  startWork,
			})
		}

	}

	sort.Slice(commitDates, func(i, j int) bool {
		return commitDates[i].StartWork.Before(commitDates[j].StartWork)
	})

	workIntervals := []WorkInterval{
		{
			Start: commitDates[0].StartWork,
			End:   commitDates[0].FinishWork,
			Infos: []CommitInfo{commitDates[0]},
		},
	}
	for i := 1; i < len(commitDates); i++ {
		curStart := commitDates[i].StartWork
		end := commitDates[i].FinishWork

		j := len(workIntervals) - 1
		if workIntervals[j].End.After(curStart) {
			workIntervals[j].End = end
			workIntervals[j].Infos = append(workIntervals[j].Infos, commitDates[i])
		} else {
			workIntervals = append(workIntervals, WorkInterval{
				Start: curStart,
				End:   end,
				Infos: []CommitInfo{commitDates[i]},
			})
		}
	}

	dateStr, err := json.Marshal(commitDates)
	if err != nil {
		log.Fatal(err)
	}
	os.WriteFile("work_times.txt", dateStr, 0666)

	workStr, err := json.Marshal(workIntervals)
	if err != nil {
		log.Fatal(err)
	}
	os.WriteFile("work_intervals.txt", workStr, 0666)

	//lets create day, aligh it to 15 minutes and calculates wasted time...
	endSleepI := 4 * 8    //until 8AM
	startSleepI := 4 * 23 //from 23AM

	moveInterval := time.Minute * 15
	dayInterval := time.Hour * 24

	theDay := workIntervals[0].Start
	theDay = time.Date(
		theDay.Year(),
		theDay.Month(),
		theDay.Day(),
		0,
		0,
		0,
		0,
		theDay.Location(),
	)
	nextDay := theDay.Add(dayInterval)

	var workDays []WorkDay
	k := 0
	for i := 0; i < len(workIntervals); {
		workDays = append(workDays, WorkDay{})
		curDate := theDay

		work := 0
		waste := 0
		for j := 0; j < 24*4; j++ {
			info := ChartInto{
				Date:  curDate,
				Value: 0,
			}

			if j <= endSleepI {
				info.Value = -5
			} else if j > startSleepI {
				info.Value = -5
			} else if i < len(workIntervals) && workIntervals[i].End.Before(nextDay) {
				if InTimeSpan(workIntervals[i].Start, workIntervals[i].End, curDate) {
					info.Value = 5
					work++
				} else if workIntervals[i].End.Before(curDate) { //when outside of range move
					i++
				}
			} else {
				waste++
				info.Value = 0
			}
			workDays[k].DayIn15Minuts = append(workDays[k].DayIn15Minuts, info)

			curDate = curDate.Add(moveInterval)
		}
		workDays[k].Sleep = 9 * 60
		workDays[k].Work = work * 15
		workDays[k].Waste = waste * 15
		workDays[k].WorkToWaste = float64(work) / float64(waste)
		k++
		theDay = curDate
		nextDay = theDay.Add(dayInterval)
	}

	os.WriteFile("days.json", JsonIdent(workDays), 0666)

}

// Inclusive (start, end)
func InTimeSpan(start, end, check t.Time) bool {
	startUnix := start.Add(-15 * t.Minute).UnixMilli()
	endUnix := end.Add(15 * time.Minute).UnixMilli()
	if startUnix > endUnix {
		log.Fatalf("InTimeSpan endUnix > startUnix : %v -> %v", start, end)
	}
	checkUnix := check.UnixMilli()

	diffStart := checkUnix - startUnix
	diffEnd := checkUnix - endUnix

	return diffStart >= 0 && diffEnd <= 0
}

func JsonIdent(t interface{}) []byte {
	jb, err := json.MarshalIndent(t, "", "    ")
	if err != nil {
		log.Fatalf("JsonB: %v", err)
	}
	return jb
}
