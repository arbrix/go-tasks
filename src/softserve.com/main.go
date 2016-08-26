package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func readTeams(fileName string) *map[string][]string {
	var result = make(map[string][]string)
	file, err := os.Open(fileName)
	if err != nil {
		println(err.Error())
		return nil
	}
	defer file.Close()
	var reader = bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasSuffix(line, "\n") {
			line = line[:len(line)-1]
		}
		var teamParts = strings.SplitN(line, ":", 2)
		if len(teamParts) < 2 {
			println("Team is undefined. Row is skipped")
		}
		var team = strings.Trim(teamParts[0], " ")
		result[team] = append(*new([]string), splitLine(teamParts[1], ",")...)
	}
	return &result
}

func scanTeams(fileName string) *map[string][]string {
	var result = make(map[string][]string)
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		println(err.Error())
		return nil
	}
	scaner := bufio.NewScanner(bufio.NewReader(file))
	scaner.Split(bufio.ScanLines)
	for scaner.Scan() {
		var line = scaner.Text()
		var teamParts = strings.SplitN(line, ":", 2)
		if len(teamParts) < 2 {
			println("Team is undefined. Row is skipped")
		}
		var team = strings.Trim(teamParts[0], " ")
		result[team] = append(*new([]string), splitLine(teamParts[1], ",")...)
	}
	return &result
}

func splitLine(line, separator string) []string {
	var tokens = strings.Split(line, separator)
	for i := range tokens {
		tokens[i] = strings.Trim(tokens[i], " \t")
	}
	return tokens
}

func getScores(player string) (int, error) {
	var reqStr = fmt.Sprintf("http://localhost:8080/GameService/getScore/%s", player)
	resp, err := http.Get(reqStr)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	var buffer = make([]byte, 100, 100)
	read, err := resp.Body.Read(buffer)
	if err != nil && err != io.EOF {
		return -2, err
	}
	if read > 0 {
		s, err := strconv.ParseInt(string(buffer[:read]), 0, 0)
		if err != nil {
			return -3, err
		}
		return int(s), nil
	}
	return 0, errors.New("Response is empty")
}

type playerResult struct {
	Team  string
	Name  string
	Score int
	Error error
}

type playerResultSorter struct{ items *[]playerResult }

func (pr playerResultSorter) Len() int {
	return len(*pr.items)
}

func (pr playerResultSorter) Less(i1, i2 int) bool {
	return (*pr.items)[i1].Score < (*pr.items)[i2].Score
}

func (pr playerResultSorter) Swap(i1, i2 int) {
	var result = (*pr.items)[i1]
	(*pr.items)[i1] = (*pr.items)[i2]
	(*pr.items)[i2] = result
}

func play(teams *map[string][]string) <-chan playerResult {
	var out = make(chan playerResult)
	for team := range *teams {
		for _, player := range (*teams)[team] {
			var player = player
			var team = team
			go func() {
				scores, e := getScores(player)
				out <- playerResult{team, player, scores, e}
			}()
		}
	}
	return out
}

func findWiner(results *[]playerResult) (individualResults *[]playerResult, teamResults map[string]int) {
	var intermediateResults = make([]playerResult, len(*results))
	individualResults = &intermediateResults
	copy(intermediateResults, *results)
	sort.Sort(sort.Reverse(playerResultSorter{individualResults}))
	teamResults = make(map[string]int)
	for _, player := range *results {
		var _, exists = teamResults[player.Team]
		if !exists {
			teamResults[player.Team] = player.Score
		} else {
			teamResults[player.Team] += player.Score
		}
	}
	return
}

func waitAndSummarize(in <-chan playerResult, playerNumber int) (individualWinners *[]playerResult, teamWinners map[string]int) {
	var counter = 0
	var stopTimer = time.NewTimer(time.Minute * 2)
	var results = *new([]playerResult)

summarize:
	for {
		select {
		case p := <-in:
			counter++
			results = append(results, p)
			stopTimer.Reset(time.Minute * 2)
			if counter == playerNumber {
				println("All players are done. Exit.")
				break summarize
			}
		case <-stopTimer.C:
			println("Time out. Exit.")
			break summarize
		}
	}
	individualWinners, teamWinners = findWiner(&results)
	return
}

func main() {
	var teams1 = readTeams("testData/Teams.txt")
	var playerCount int
	for team := range *teams1 {
		playerCount += len((*teams1)[team])
	}
	in := play(teams1)
	individualWinners, teamWinners := waitAndSummarize(in, playerCount)
	var wonTeam string
	var teamScore int
	for t := range teamWinners {
		if teamWinners[t] > teamScore {
			teamScore = teamWinners[t]
			wonTeam = t
		}
	}
	fmt.Printf("Won player %v, won team %s\n", (*individualWinners)[0], wonTeam)
}
